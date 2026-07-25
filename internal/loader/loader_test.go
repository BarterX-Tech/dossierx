// This file was previously entirely missing: internal/loader — the one
// package that actually touches the filesystem for claim content — had
// zero test coverage. It covers LoadClaims/SaveClaim/FindByID directly,
// plus the "claims file with valid YAML but wrong top-level shape" edge
// case (a claim file that decodes as a YAML sequence or scalar instead of
// a map), which no other package's tests exercised at the loader level
// (only indirectly, via the CLI, in tests/).
package loader

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func writeFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestLoadClaims_Basic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nbody: claim a\ngoverned_by:\n  type: none\n  reason: fixture\n")
	writeFile(t, dir, "b.yml", "id: widget.contract.b\nfacet: contract\nmodule: widget\nstatus: draft\nbody: claim b\ngoverned_by:\n  type: none\n  reason: fixture\n")
	// Non-YAML files must be ignored.
	writeFile(t, dir, "README.md", "not a claim")

	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("expected 2 claims, got %d: %+v", len(claims), claims)
	}
	// Sorted by SourcePath: a.yaml before b.yml.
	if !strings.HasSuffix(claims[0].SourcePath, "a.yaml") {
		t.Errorf("expected claims[0] to be from a.yaml, got %s", claims[0].SourcePath)
	}
	if !strings.HasSuffix(claims[1].SourcePath, "b.yml") {
		t.Errorf("expected claims[1] to be from b.yml, got %s", claims[1].SourcePath)
	}
	if claims[0].ID != "widget.contract.a" || claims[1].ID != "widget.contract.b" {
		t.Errorf("unexpected ids: %q, %q", claims[0].ID, claims[1].ID)
	}
}

func TestLoadClaims_RecursesSubdirectories(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "nested", "deeper")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, sub, "c.yaml", "id: widget.contract.c\nfacet: contract\nmodule: widget\nstatus: draft\nbody: nested claim\ngoverned_by:\n  type: none\n  reason: fixture\n")

	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	if len(claims) != 1 || claims[0].ID != "widget.contract.c" {
		t.Fatalf("expected the nested claim to be found, got %+v", claims)
	}
}

func TestLoadClaims_DirDoesNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := LoadClaims(missing)
	if err == nil {
		t.Fatal("expected error for a claims_dir that does not exist, got nil")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("expected error to name the missing dir %q, got: %v", missing, err)
	}
}

func TestLoadClaims_DirIsAFile(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "not-a-dir", "x")
	_, err := LoadClaims(file)
	if err == nil {
		t.Fatal("expected error when claims_dir is a file, not a directory, got nil")
	}
}

func TestLoadClaims_UnknownFieldIsStrictError(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.yaml", "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nbody: x\nsome_typo_field: true\ngoverned_by:\n  type: none\n  reason: fixture\n")
	_, err := LoadClaims(dir)
	if err == nil {
		t.Fatal("expected strict-decode error for an unknown claim field, got nil")
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("expected error to name the file %q, got: %v", p, err)
	}
}

// TestLoadClaims_ValidYAMLWrongTopLevelShape covers a claim file that is
// syntactically valid YAML but not shaped like a claim at all: a sequence
// at the top level (e.g. someone pasted a list of claims into one file
// instead of one claim per file) must be a clear, file-naming error, not a
// panic or a silently-empty claim.
func TestLoadClaims_ValidYAMLWrongTopLevelShape(t *testing.T) {
	cases := []struct {
		name     string
		contents string
	}{
		{"sequence", "- id: widget.contract.a\n- id: widget.contract.b\n"},
		{"scalar_string", "just a plain string, not a claim\n"},
		{"scalar_number", "42\n"},
		{"null_document", "null\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := writeFile(t, dir, "bad.yaml", tc.contents)

			claims, err := LoadClaims(dir)
			if tc.name == "null_document" {
				// An empty/null YAML document decodes to a zero-value
				// Claim (id ""); this is not a shape error at the loader
				// level (lint's id-shape rule is what would ultimately
				// reject the empty id) — assert it doesn't panic and
				// produces exactly one (empty) claim.
				if err != nil {
					t.Fatalf("LoadClaims on a null document: unexpected error: %v", err)
				}
				if len(claims) != 1 {
					t.Fatalf("expected 1 (empty) claim from a null document, got %d", len(claims))
				}
				return
			}
			if err == nil {
				t.Fatalf("expected LoadClaims to error on wrong top-level shape (%s), got nil, claims: %+v", tc.name, claims)
			}
			if !strings.Contains(err.Error(), p) {
				t.Errorf("expected error to name the offending file %q, got: %v", p, err)
			}
		})
	}
}

// TestLoadClaims_MultiDocumentIsError covers a file that stacks more than
// one YAML document (---separated) into a single claim file. LoadClaims
// previously read only the first document and silently dropped the rest;
// because SaveClaim rewrites a claim's file as a single document, a later
// lock/reaudit would clobber those dropped file-siblings. This must now be
// a hard error naming the offending file, while a legitimate one-document
// file (even with a leading/trailing document marker) still loads.
func TestLoadClaims_MultiDocumentIsError(t *testing.T) {
	t.Run("two documents in one file is an error naming the file", func(t *testing.T) {
		dir := t.TempDir()
		p := writeFile(t, dir, "two.yaml",
			"id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nbody: claim a\ngoverned_by:\n  type: none\n  reason: fixture\n"+
				"---\n"+
				"id: widget.contract.b\nfacet: contract\nmodule: widget\nstatus: draft\nbody: claim b\ngoverned_by:\n  type: none\n  reason: fixture\n")
		_, err := LoadClaims(dir)
		if err == nil {
			t.Fatal("expected an error for a file with two YAML documents, got nil")
		}
		if !strings.Contains(err.Error(), p) {
			t.Errorf("expected error to name the offending file %q, got: %v", p, err)
		}
	})

	t.Run("a single document still loads exactly as before", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "one.yaml",
			"---\nid: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nbody: claim a\ngoverned_by:\n  type: none\n  reason: fixture\n")
		claims, err := LoadClaims(dir)
		if err != nil {
			t.Fatalf("LoadClaims on a single-document file: unexpected error: %v", err)
		}
		if len(claims) != 1 || claims[0].ID != "widget.contract.a" {
			t.Fatalf("expected exactly one claim widget.contract.a, got %+v", claims)
		}
	})
}

func TestSaveClaim_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.yaml", "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nbody: original\ngoverned_by:\n  type: none\n  reason: fixture\n")

	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	claim := claims[0]
	claim.Body = "edited body"
	claim.Status = model.StatusLocked

	if err := SaveClaim(claim); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}

	reloaded, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims after save: %v", err)
	}
	if len(reloaded) != 1 {
		t.Fatalf("expected 1 claim after save, got %d", len(reloaded))
	}
	if reloaded[0].Body != "edited body" {
		t.Errorf("expected saved body to round-trip, got %q", reloaded[0].Body)
	}
	if reloaded[0].Status != model.StatusLocked {
		t.Errorf("expected saved status to round-trip, got %q", reloaded[0].Status)
	}
	_ = p
}

func TestLoadClaims_OrderFieldRoundTrips(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nbody: claim a\norder: 5\ngoverned_by:\n  type: none\n  reason: fixture\n")
	// A claim that omits order entirely must decode with the zero value
	// (unset), not error — order is optional.
	writeFile(t, dir, "b.yaml", "id: widget.contract.b\nfacet: contract\nmodule: widget\nstatus: draft\nbody: claim b\ngoverned_by:\n  type: none\n  reason: fixture\n")

	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("expected 2 claims, got %d", len(claims))
	}
	if claims[0].Order != 5 {
		t.Errorf("expected claims[0].Order == 5, got %d", claims[0].Order)
	}
	if claims[1].Order != 0 {
		t.Errorf("expected claims[1].Order to default to 0 when omitted, got %d", claims[1].Order)
	}

	// SaveClaim -> LoadClaims must preserve an explicit Order.
	claim := claims[0]
	claim.Order = 7
	if err := SaveClaim(claim); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}
	reloaded, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims after save: %v", err)
	}
	var got model.Claim
	for _, c := range reloaded {
		if c.ID == "widget.contract.a" {
			got = c
		}
	}
	if got.Order != 7 {
		t.Errorf("expected saved Order 7 to round-trip, got %d", got.Order)
	}
}

func TestSaveClaim_NoSourcePathIsError(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.a", Facet: "contract", Module: "widget", Status: model.StatusDraft}
	err := SaveClaim(claim)
	if err == nil {
		t.Fatal("expected SaveClaim to error when claim.SourcePath is empty, got nil")
	}
	if !strings.Contains(err.Error(), claim.ID) {
		t.Errorf("expected error to name the claim id %q, got: %v", claim.ID, err)
	}
}

func TestFindByID(t *testing.T) {
	claims := []model.Claim{
		{ID: "widget.contract.a"},
		{ID: "widget.contract.b"},
	}
	got, ok := FindByID(claims, "widget.contract.b")
	if !ok || got.ID != "widget.contract.b" {
		t.Fatalf("FindByID(b) = %+v, %v; want widget.contract.b, true", got, ok)
	}
	_, ok = FindByID(claims, "widget.contract.missing")
	if ok {
		t.Fatalf("FindByID(missing) = _, true; want false")
	}
}

func TestLoadClaims_SectionFieldRoundTrips(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", "id: widget.contract.a\nfacet: contract\nstatus: draft\nbody: claim a\nsection: 5 - workflows / lifecycle\ngoverned_by:\n  type: none\n  reason: fixture\n")
	// A claim that omits section entirely must decode with the zero value
	// (unset), not error — section is optional and project-agnostic.
	writeFile(t, dir, "b.yaml", "id: widget.contract.b\nfacet: contract\nstatus: draft\nbody: claim b\ngoverned_by:\n  type: none\n  reason: fixture\n")

	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	var a, b model.Claim
	for _, c := range claims {
		switch c.ID {
		case "widget.contract.a":
			a = c
		case "widget.contract.b":
			b = c
		}
	}
	if a.Section != "5 - workflows / lifecycle" {
		t.Errorf("expected claim a Section to load, got %q", a.Section)
	}
	if b.Section != "" {
		t.Errorf("expected claim b Section to default to empty when omitted, got %q", b.Section)
	}

	a.Section = "6 - later"
	if err := SaveClaim(a); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}
	reloaded, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims after save: %v", err)
	}
	got, ok := FindByID(reloaded, "widget.contract.a")
	if !ok {
		t.Fatal("expected widget.contract.a to still be found after save")
	}
	if got.Section != "6 - later" {
		t.Errorf("expected saved Section %q to round-trip, got %q", "6 - later", got.Section)
	}
}

func TestLoadClaims_RawHTMLFieldsAndMockupLayoutRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", "id: widget.contract.a\nfacet: contract\nstatus: draft\nlayout: mockup\nraw_html: \"<div>mock</div>\"\nraw_html_reviewed: true\ngoverned_by:\n  type: none\n  reason: fixture\n")
	// A claim that omits raw_html/raw_html_reviewed entirely must decode
	// with the zero value (unset/false), not error — both are optional.
	writeFile(t, dir, "b.yaml", "id: widget.contract.b\nfacet: contract\nstatus: draft\nbody: claim b\ngoverned_by:\n  type: none\n  reason: fixture\n")

	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	var a, b model.Claim
	for _, c := range claims {
		switch c.ID {
		case "widget.contract.a":
			a = c
		case "widget.contract.b":
			b = c
		}
	}
	if a.Layout != model.LayoutMockup {
		t.Errorf("expected claim a Layout == LayoutMockup, got %q", a.Layout)
	}
	if a.RawHTML != "<div>mock</div>" {
		t.Errorf("expected claim a RawHTML to load, got %q", a.RawHTML)
	}
	if !a.RawHTMLReviewed {
		t.Errorf("expected claim a RawHTMLReviewed == true, got false")
	}
	if b.RawHTML != "" || b.RawHTMLReviewed {
		t.Errorf("expected claim b raw_html fields to default to unset, got RawHTML=%q RawHTMLReviewed=%v", b.RawHTML, b.RawHTMLReviewed)
	}

	a.RawHTMLReviewed = false
	if err := SaveClaim(a); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}
	reloaded, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims after save: %v", err)
	}
	got, ok := FindByID(reloaded, "widget.contract.a")
	if !ok {
		t.Fatal("expected widget.contract.a to still be found after save")
	}
	if got.RawHTMLReviewed {
		t.Errorf("expected saved RawHTMLReviewed=false to round-trip, got true")
	}
	if got.RawHTML != "<div>mock</div>" {
		t.Errorf("expected RawHTML to survive a save that only touched RawHTMLReviewed, got %q", got.RawHTML)
	}
}

// TestLoadClaims_TableRowsPreserveAuthoredColumnOrder is the row-ordering
// fix's end-to-end regression test: a table claim's rows use a column
// order that is not alphabetically sortable (zeta, alpha, middle), so a
// regression back to Go's old map-then-sort-alphabetical behavior (which
// would produce alpha, middle, zeta) is caught. It exercises the real
// on-disk decode path (LoadClaims), not just the model package's direct
// yaml.Unmarshal, and also checks SaveClaim writes the same authored order
// back out rather than losing it.
func TestLoadClaims_TableRowsPreserveAuthoredColumnOrder(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.yaml", "id: widget.contract.a\nfacet: contract\nstatus: draft\nlayout: table\n"+
		"rows:\n  - zeta: 1\n    alpha: 2\n    middle: 3\n  - zeta: 4\n    alpha: 5\n    middle: 6\n"+
		"governed_by:\n  type: none\n  reason: fixture\n")

	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	if len(claims) != 1 || len(claims[0].Rows) != 2 {
		t.Fatalf("expected 1 claim with 2 rows, got %+v", claims)
	}

	wantOrder := []string{"zeta", "alpha", "middle"}
	for i, row := range claims[0].Rows {
		got := model.RowColumns(row)
		if len(got) != len(wantOrder) {
			t.Fatalf("row %d authored column order = %v, want %v", i, got, wantOrder)
		}
		for j := range wantOrder {
			if got[j] != wantOrder[j] {
				t.Fatalf("row %d authored column order = %v, want %v (this must be the YAML's authored order, not alphabetical)", i, got, wantOrder)
			}
		}
	}

	// SaveClaim must write the rows back out in the same authored order,
	// not alphabetically re-sorted, and reloading must still see it.
	if err := SaveClaim(claims[0]); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}
	saved, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	firstRowIdx := strings.Index(string(saved), "rows:")
	if firstRowIdx < 0 {
		t.Fatalf("saved file has no rows: section:\n%s", saved)
	}
	rowsSection := string(saved)[firstRowIdx:]
	zetaIdx := strings.Index(rowsSection, "zeta:")
	alphaIdx := strings.Index(rowsSection, "alpha:")
	middleIdx := strings.Index(rowsSection, "middle:")
	if !(zetaIdx >= 0 && zetaIdx < alphaIdx && alphaIdx < middleIdx) {
		t.Fatalf("saved rows: section did not preserve authored column order zeta,alpha,middle:\n%s", rowsSection)
	}

	reloaded, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims after save: %v", err)
	}
	got := model.RowColumns(reloaded[0].Rows[0])
	for j := range wantOrder {
		if got[j] != wantOrder[j] {
			t.Fatalf("reloaded row 0 authored column order = %v, want %v", got, wantOrder)
		}
	}
}

// TestLoadClaims_CommentsRoundTrip proves a fully-populated comment thread
// (id, status, author, created, body, edited, a reply with its own id, and
// resolve metadata) survives LoadClaims -> SaveClaim -> LoadClaims unchanged.
func TestLoadClaims_CommentsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := `id: widget.contract.a
facet: contract
module: widget
status: locked
build_role: schema
body: claim a
comments:
  - id: c-8f3a2b
    status: resolved
    author: human
    created: 2026-07-24T10:12:00Z
    body: |
      This row contradicts the API facet — which is right?
    edited: true
    replies:
      - id: r-4c9e11
        author: agent
        created: 2026-07-24T10:40:00Z
        body: Fixed the rows; API facet was stale.
        edited: false
    resolved_by: human
    resolved_at: 2026-07-24T11:02:00Z
governed_by:
  type: none
  reason: fixture
`
	writeFile(t, dir, "a.yaml", src)

	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	c := claims[0]
	if len(c.Comments) != 1 {
		t.Fatalf("expected 1 comment thread, got %d", len(c.Comments))
	}
	th := c.Comments[0]
	if th.ID != "c-8f3a2b" || th.Status != model.CommentStatusResolved || th.Author != model.CommentRoleHuman {
		t.Fatalf("thread header wrong: %+v", th)
	}
	if th.ResolvedBy != model.CommentRoleHuman || th.ResolvedAt != "2026-07-24T11:02:00Z" || !th.Edited {
		t.Fatalf("thread resolve/edit metadata wrong: %+v", th)
	}
	if len(th.Replies) != 1 || th.Replies[0].ID != "r-4c9e11" || th.Replies[0].Author != model.CommentRoleAgent {
		t.Fatalf("reply wrong: %+v", th.Replies)
	}

	// Save and reload: Comments must be identical.
	if err := SaveClaim(c); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}
	reloaded, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims after save: %v", err)
	}
	if !reflect.DeepEqual(reloaded[0].Comments, c.Comments) {
		t.Fatalf("comments did not round-trip through SaveClaim:\n want %#v\n  got %#v", c.Comments, reloaded[0].Comments)
	}
}

// TestSaveClaim_CommentFreeStaysByteIdentical is the omitempty byte-identity
// guarantee: a claim that has never been commented on writes no comments: key,
// and clearing every comment returns the file byte-for-byte to that
// comment-free form (so adding the feature never rewrites an uncommented claim).
func TestSaveClaim_CommentFreeStaysByteIdentical(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nbody: claim a\ngoverned_by:\n  type: none\n  reason: fixture\n")
	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	c := claims[0]

	if err := SaveClaim(c); err != nil {
		t.Fatalf("SaveClaim (comment-free): %v", err)
	}
	want, err := os.ReadFile(c.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(want, []byte("comments:")) {
		t.Fatalf("a comment-free claim must not write a comments: key, got:\n%s", want)
	}

	// Add a comment: the key now appears.
	c.Comments = []model.Comment{{ID: "c-000001", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-24T10:12:00Z", Body: "x"}}
	if err := SaveClaim(c); err != nil {
		t.Fatalf("SaveClaim (with comment): %v", err)
	}
	withC, err := os.ReadFile(c.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(withC, []byte("comments:")) {
		t.Fatalf("expected a comments: key when a comment is present, got:\n%s", withC)
	}

	// Clear every comment (len-0 slice): omitempty drops the key again, byte-identical.
	c.Comments = c.Comments[:0]
	if err := SaveClaim(c); err != nil {
		t.Fatalf("SaveClaim (comments cleared): %v", err)
	}
	got, err := os.ReadFile(c.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("clearing all comments must return the file byte-identical to the comment-free form\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// TestLoadClaims_UnknownCommentField_StrictError proves KnownFields(true)
// strict decode rejects a misspelled field INSIDE a comment, not just at the
// top level — so the struct field + FORMAT.md schema doc must land together.
func TestLoadClaims_UnknownCommentField_StrictError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", `id: widget.contract.a
facet: contract
module: widget
status: draft
body: claim a
comments:
  - id: c-000001
    status: open
    author: human
    created: 2026-07-24T10:12:00Z
    body: x
    edited: false
    bodyy: misspelled
governed_by:
  type: none
  reason: fixture
`)
	_, err := LoadClaims(dir)
	if err == nil {
		t.Fatal("expected a strict-decode error for an unknown comment field, got nil")
	}
	if !strings.Contains(err.Error(), "bodyy") {
		t.Fatalf("expected the error to name the unknown field 'bodyy', got: %v", err)
	}
}
