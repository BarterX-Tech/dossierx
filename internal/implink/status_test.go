package implink

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestStatus_MissingArtifact_WrapsErrNoArtifact(t *testing.T) {
	cfg := testConfig(t, "widget")
	_, err := Status(nil, cfg, "widget")
	if err == nil || !errors.Is(err, ErrNoArtifact) {
		t.Fatalf("expected an ErrNoArtifact-wrapping error, got: %v", err)
	}
}

func TestStatus_ReportsLinkedCountAndNoDriftWhenFileUnchanged(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	file := writeSourceFile(t, cfg, "a.go", "package widget")
	if _, err := Set(claims, cfg, "widget", "widget.contract.main", file, ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	report, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if report.LinkedClaims != 1 {
		t.Fatalf("expected 1 linked claim, got %d", report.LinkedClaims)
	}
	if len(report.Drifted) != 0 {
		t.Fatalf("expected no drift for an unchanged file, got %+v", report.Drifted)
	}
}

func TestStatus_DetectsDriftAfterFileMutates(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	file := writeSourceFile(t, cfg, "a.go", "package widget // v1")
	if _, err := Set(claims, cfg, "widget", "widget.contract.main", file, ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	full := filepath.Join(cfg.Dir(), file)
	if err := os.WriteFile(full, []byte("package widget // v2, mutated"), 0o644); err != nil {
		t.Fatalf("mutate file: %v", err)
	}

	report, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(report.Drifted) != 1 {
		t.Fatalf("expected exactly 1 drifted entry after mutating the linked file, got %+v", report.Drifted)
	}
	got := report.Drifted[0]
	if got.ClaimID != "widget.contract.main" || got.File != file {
		t.Fatalf("unexpected drift entry: %+v", got)
	}
	if got.Reason == "" {
		t.Fatalf("expected a non-empty human-readable drift reason")
	}
}

func TestStatus_DetectsDriftWhenFileDeleted(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	file := writeSourceFile(t, cfg, "a.go", "package widget")
	if _, err := Set(claims, cfg, "widget", "widget.contract.main", file, ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := os.Remove(filepath.Join(cfg.Dir(), file)); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	report, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(report.Drifted) != 1 {
		t.Fatalf("expected the deleted file reported as drifted, got %+v", report.Drifted)
	}
}

// ---------------------------------------------------------------------
// Unlinked counting
// ---------------------------------------------------------------------

func TestStatus_UnlinkedCounting_OnlyCodeProducingPhasesCount(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{
		lockedClaim("widget.contract.orientation", "widget", model.BuildRoleOrientation),
		lockedClaim("widget.contract.outofscope", "widget", model.BuildRoleOutOfScope),
		lockedClaim("widget.contract.schema", "widget", model.BuildRoleSchema),
		lockedClaim("widget.contract.behavior", "widget", model.BuildRoleBehavior),
		lockedClaim("widget.contract.api", "widget", model.BuildRoleAPI),
		lockedClaim("widget.contract.verify", "widget", model.BuildRoleVerification),
	}
	// Link only the schema claim; everything else in a code-producing phase
	// stays unlinked.
	file := writeSourceFile(t, cfg, "schema.go", "package widget")
	if _, err := Set(claims, cfg, "widget", "widget.contract.schema", file, ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	report, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	// behavior, api, verify are unlinked; orientation and out-of-scope must
	// never count even though they too have no linked file.
	if report.UnlinkedCount != 3 {
		t.Fatalf("expected 3 unlinked code-producing-phase claims, got %d (%v)", report.UnlinkedCount, report.UnlinkedIDs)
	}
	for _, id := range []string{"widget.contract.orientation", "widget.contract.outofscope"} {
		for _, u := range report.UnlinkedIDs {
			if u == id {
				t.Fatalf("did not expect %q counted as unlinked (not a code-producing phase)", id)
			}
		}
	}
}

func TestStatus_UnlinkedCounting_DraftClaimsNeverCount(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{
		{ID: "widget.contract.draft", Module: "widget", Status: model.StatusDraft, BuildRole: model.BuildRoleBehavior},
	}
	file := writeSourceFile(t, cfg, "a.go", "package widget")
	locked := []model.Claim{lockedClaim("widget.contract.locked", "widget", model.BuildRoleSchema)}
	if _, err := Set(locked, cfg, "widget", "widget.contract.locked", file, ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	report, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if report.UnlinkedCount != 0 {
		t.Fatalf("expected a draft claim to never count as unlinked, got %d (%v)", report.UnlinkedCount, report.UnlinkedIDs)
	}
}

func TestStatus_Summary_Format(t *testing.T) {
	r := &StatusReport{LinkedClaims: 2, Drifted: []DriftEntry{{}}, UnlinkedCount: 3}
	want := "impl-links: 2 linked, 1 drifted, 3 unlinked-in-schema/behavior/api/verification-phases"
	if got := r.Summary(); got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------
// ViewsByClaim
// ---------------------------------------------------------------------

func TestViewsByClaim_MissingArtifact_WrapsErrNoArtifact(t *testing.T) {
	cfg := testConfig(t, "widget")
	_, err := ViewsByClaim(cfg, "widget")
	if err == nil || !errors.Is(err, ErrNoArtifact) {
		t.Fatalf("expected an ErrNoArtifact-wrapping error, got: %v", err)
	}
}

func TestViewsByClaim_MarksDriftedFilePerEntry(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	fileA := writeSourceFile(t, cfg, "a.go", "package widget // a")
	fileB := writeSourceFile(t, cfg, "b.go", "package widget // b")
	if _, err := Set(claims, cfg, "widget", "widget.contract.main", fileA, "FuncA"); err != nil {
		t.Fatalf("Set a: %v", err)
	}
	if _, err := Set(claims, cfg, "widget", "widget.contract.main", fileB, "FuncB"); err != nil {
		t.Fatalf("Set b: %v", err)
	}

	// Mutate only fileA.
	if err := os.WriteFile(filepath.Join(cfg.Dir(), fileA), []byte("mutated"), 0o644); err != nil {
		t.Fatalf("mutate: %v", err)
	}

	views, err := ViewsByClaim(cfg, "widget")
	if err != nil {
		t.Fatalf("ViewsByClaim: %v", err)
	}
	files := views["widget.contract.main"]
	if len(files) != 2 {
		t.Fatalf("expected 2 files in view, got %d", len(files))
	}
	byFile := map[string]ViewFile{}
	for _, f := range files {
		byFile[f.File] = f
	}
	if !byFile[fileA].Drifted {
		t.Fatalf("expected fileA marked drifted, got %+v", byFile[fileA])
	}
	if byFile[fileB].Drifted {
		t.Fatalf("expected fileB (unchanged) not marked drifted, got %+v", byFile[fileB])
	}
	if byFile[fileA].Symbol != "FuncA" || byFile[fileB].Symbol != "FuncB" {
		t.Fatalf("expected symbols preserved in view, got %+v", files)
	}
}
