package loader

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// SaveClaimIfUnchanged is the optimistic-concurrency backstop layered under
// the project-wide claims sentinel: it writes a claim back only if the file
// still holds the exact bytes the caller captured (via CaptureClaimFileToken)
// at load time, otherwise refuses with a matchable ErrClaimFileChanged an API
// can surface as HTTP 409. These tests pin that contract.

func TestSaveClaimIfUnchanged_WritesWhenFileUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("id: m.f.x\nfacet: f\nstatus: draft\nbody: |\n  original.\n"), 0o644); err != nil {
		t.Fatalf("seed claim file: %v", err)
	}

	token, err := CaptureClaimFileToken(path)
	if err != nil {
		t.Fatalf("capture token: %v", err)
	}

	c := model.Claim{ID: "m.f.x", Facet: "f", Status: model.StatusDraft, Body: "updated body.\n", SourcePath: path}
	if err := SaveClaimIfUnchanged(c, token); err != nil {
		t.Fatalf("SaveClaimIfUnchanged returned an error on an unchanged file: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "updated body.") {
		t.Fatalf("expected the file to be rewritten with the mutated claim, got:\n%s", got)
	}
}

func TestSaveClaimIfUnchanged_RefusesWhenFileChangedUnderneath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("id: m.f.x\nstatus: draft\nbody: |\n  original.\n"), 0o644); err != nil {
		t.Fatalf("seed claim file: %v", err)
	}

	token, err := CaptureClaimFileToken(path)
	if err != nil {
		t.Fatalf("capture token: %v", err)
	}

	// Another writer changes the file after we captured the token but before
	// we save — exactly the out-of-band edit the backstop exists to catch.
	outOfBand := []byte("id: m.f.x\nstatus: locked\nbody: |\n  someone else got here first.\n")
	if err := os.WriteFile(path, outOfBand, 0o644); err != nil {
		t.Fatalf("out-of-band write: %v", err)
	}

	c := model.Claim{ID: "m.f.x", Status: model.StatusDraft, Body: "our stale update.\n", SourcePath: path}
	err = SaveClaimIfUnchanged(c, token)
	if !errors.Is(err, ErrClaimFileChanged) {
		t.Fatalf("expected ErrClaimFileChanged, got: %v", err)
	}

	// The refused write must not have clobbered the out-of-band content.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(outOfBand) {
		t.Fatalf("expected the out-of-band content preserved after a refused write, got:\n%s", got)
	}
}

func TestSaveClaimIfUnchanged_ErrorsWithoutSourcePath(t *testing.T) {
	err := SaveClaimIfUnchanged(model.Claim{ID: "m.f.x"}, ClaimFileToken{})
	if err == nil {
		t.Fatal("expected an error when the claim has no source path")
	}
	if errors.Is(err, ErrClaimFileChanged) {
		t.Fatalf("a missing source path is not a file-changed conflict, got: %v", err)
	}
}

func TestCaptureClaimFileToken_ErrorsOnMissingFile(t *testing.T) {
	if _, err := CaptureClaimFileToken(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected an error capturing a token for a nonexistent file")
	}
}
