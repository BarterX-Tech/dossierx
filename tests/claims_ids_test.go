// This file covers the "Claims & IDs" edge-case category end-to-end at the
// CLI level: how "dossierx lint"/"dossierx check" behave for malformed, duplicate,
// empty, or oddly-shaped claim ids and claim content. It reuses the
// binPath/run/writeFixtureProject scaffolding from cli_test.go in this same
// package.
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeIDsFixtureProject writes a minimal project.config.yaml (facets:
// [contract, internals], modules: [widget]) plus an initially empty
// claims/ directory into root.
func writeIDsFixtureProject(t *testing.T, root string) (claimsDir string) {
	t.Helper()
	claimsDir = filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\n" +
		"facets:\n  - contract\n  - internals\n" +
		"modules:\n  - widget\n" +
		"claims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	return claimsDir
}

func writeIDsClaim(t *testing.T, claimsDir, filename, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(claimsDir, filename), []byte(contents), 0o644); err != nil {
		t.Fatalf("write claim %s: %v", filename, err)
	}
}

// Row 1: two claims share the same id (across files) -> ambiguous lint
// error naming the shared id (findings are reported once per claim
// carrying the duplicate, so it appears at least twice).
func TestClaimsIDs_DuplicateIDAcrossFiles(t *testing.T) {
	root := t.TempDir()
	claimsDir := writeIDsFixtureProject(t, root)

	writeIDsClaim(t, claimsDir, "a.yaml", `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: first claim with this id
governed_by:
  type: none
  reason: fixture
`)
	writeIDsClaim(t, claimsDir, "b.yaml", `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: second claim with the same id
governed_by:
  type: none
  reason: fixture
`)

	stdout, stderr, code := run(t, root, "lint")
	if code == 0 {
		t.Fatalf("expected nonzero exit for duplicate id, got 0; stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "ambiguous") {
		t.Errorf("expected ambiguous lint finding in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "widget.contract.overview") {
		t.Errorf("expected the shared id to be named in output, got:\n%s", stdout)
	}
}

// Row 2: id does not match module.facet.slug -> id-shape lint error.
func TestClaimsIDs_MalformedIDShape(t *testing.T) {
	root := t.TempDir()
	claimsDir := writeIDsFixtureProject(t, root)

	writeIDsClaim(t, claimsDir, "a.yaml", `id: widget-overview
facet: contract
module: widget
status: draft
body: id has only one segment, not module.facet.slug
governed_by:
  type: none
  reason: fixture
`)

	stdout, stderr, code := run(t, root, "lint")
	if code == 0 {
		t.Fatalf("expected nonzero exit for malformed id, got 0; stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "id-shape") {
		t.Errorf("expected id-shape lint finding in output, got:\n%s", stdout)
	}
}

// Row 3: claim's facet is not in the project's configured facets ->
// id-shape lint error naming the unknown facet.
func TestClaimsIDs_UnknownFacet(t *testing.T) {
	root := t.TempDir()
	claimsDir := writeIDsFixtureProject(t, root)

	writeIDsClaim(t, claimsDir, "a.yaml", `id: widget.doctrine.overview
facet: doctrine
module: widget
status: draft
body: doctrine is not a configured facet
governed_by:
  type: none
  reason: fixture
`)

	stdout, stderr, code := run(t, root, "lint")
	if code == 0 {
		t.Fatalf("expected nonzero exit for unknown facet, got 0; stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "id-shape") {
		t.Errorf("expected id-shape lint finding in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "doctrine") {
		t.Errorf("expected the unknown facet name %q in output, got:\n%s", "doctrine", stdout)
	}
}

// Row 4: claim's module is not in the project's configured modules ->
// id-shape lint error.
func TestClaimsIDs_UnknownModule(t *testing.T) {
	root := t.TempDir()
	claimsDir := writeIDsFixtureProject(t, root)

	writeIDsClaim(t, claimsDir, "a.yaml", `id: gadget.contract.overview
facet: contract
module: gadget
status: draft
body: gadget is not a configured module
governed_by:
  type: none
  reason: fixture
`)

	stdout, stderr, code := run(t, root, "lint")
	if code == 0 {
		t.Fatalf("expected nonzero exit for unknown module, got 0; stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "id-shape") {
		t.Errorf("expected id-shape lint finding in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "gadget") {
		t.Errorf("expected the unknown module name %q in output, got:\n%s", "gadget", stdout)
	}
}

// Row 5: claims dir is empty (zero claims) -> valid; lint, catalog, render,
// and check all succeed on nothing.
func TestClaimsIDs_EmptyClaimsDir(t *testing.T) {
	root := t.TempDir()
	writeIDsFixtureProject(t, root)
	// claims dir was created empty and never populated.

	if stdout, stderr, code := run(t, root, "lint"); code != 0 {
		t.Fatalf("lint on empty claims dir: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stdout, stderr, code := run(t, root, "catalog"); code != 0 {
		t.Fatalf("catalog on empty claims dir: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stdout, stderr, code := run(t, root, "render"); code != 0 {
		t.Fatalf("render on empty claims dir: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stdout, stderr, code := run(t, root, "check"); code != 0 {
		t.Fatalf("check on empty claims dir: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

// Row 6: a claim with neither body nor rows (nor steps) is invalid — a
// claim must carry some content, or nothing would ever render for it.
func TestClaimsIDs_NoContentIsInvalid(t *testing.T) {
	root := t.TempDir()
	claimsDir := writeIDsFixtureProject(t, root)

	writeIDsClaim(t, claimsDir, "a.yaml", `id: widget.contract.empty
facet: contract
module: widget
status: draft
governed_by:
  type: none
  reason: fixture
`)

	stdout, stderr, code := run(t, root, "lint")
	if code == 0 {
		t.Fatalf("expected nonzero exit for a claim with no body/rows/steps, got 0; stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "layout-shape-mismatch") {
		t.Errorf("expected layout-shape-mismatch finding for empty claim, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "no content") {
		t.Errorf("expected an explanatory \"no content\" message, got:\n%s", stdout)
	}
}

// Row 7: a claim with both body and rows is allowed — body is illustrative
// prose, rows is separately lint-checked structured data; carrying both is
// not a conflict.
func TestClaimsIDs_BodyAndRowsBothAllowed(t *testing.T) {
	root := t.TempDir()
	claimsDir := writeIDsFixtureProject(t, root)

	writeIDsClaim(t, claimsDir, "a.yaml", `id: widget.internals.fields
facet: internals
module: widget
status: draft
layout: table
body: |
  Prose context alongside the structured field table below.
rows:
  - field: id
    type: string
  - field: name
    type: string
governed_by:
  type: none
  reason: fixture
`)

	if stdout, stderr, code := run(t, root, "check"); code != 0 {
		t.Fatalf("expected exit 0 for a claim with both body and rows, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

// Row 8: non-ASCII/unicode in a slug is out of the id-shape grammar's
// defined character class ([a-z0-9] with single hyphens) and must be
// rejected as an id-shape error, not silently accepted.
func TestClaimsIDs_UnicodeSlugRejected(t *testing.T) {
	root := t.TempDir()
	claimsDir := writeIDsFixtureProject(t, root)

	writeIDsClaim(t, claimsDir, "a.yaml", "id: widget.contract.café-overview\n"+
		"facet: contract\n"+
		"module: widget\n"+
		"status: draft\n"+
		"body: slug contains an accented character outside [a-z0-9-]\n"+
		"governed_by:\n"+
		"  type: none\n"+
		"  reason: fixture\n")

	stdout, stderr, code := run(t, root, "lint")
	if code == 0 {
		t.Fatalf("expected nonzero exit for a unicode slug, got 0; stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "id-shape") {
		t.Errorf("expected id-shape lint finding for unicode slug, got:\n%s", stdout)
	}
}
