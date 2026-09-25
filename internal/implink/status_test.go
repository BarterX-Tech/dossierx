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
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget")}
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
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget")}
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
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget")}
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

func TestStatus_UnlinkedCounting_CodeFreeAndProjectClaimsNeverCount(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{
		codeFree(lockedClaim("widget.contract.orientation", "widget")),
		{ID: "project.outofscope", Scope: model.ScopeProject, Status: model.StatusLocked},
		lockedClaim("widget.contract.schema", "widget"),
		lockedClaim("widget.contract.behavior", "widget"),
		lockedClaim("widget.contract.api", "widget"),
		lockedClaim("widget.contract.verify", "widget"),
	}
	// Link only the schema claim; every other module claim that does not
	// declare itself code-free stays unlinked.
	file := writeSourceFile(t, cfg, "schema.go", "package widget")
	if _, err := Set(claims, cfg, "widget", "widget.contract.schema", file, ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	report, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	// behavior, api, verify are unlinked; the code-free claim and the
	// project claim must never count even though they have no linked file.
	if report.UnlinkedCount != 3 {
		t.Fatalf("expected 3 unlinked claims, got %d (%v)", report.UnlinkedCount, report.UnlinkedIDs)
	}
	for _, id := range []string{"widget.contract.orientation", "project.outofscope"} {
		for _, u := range report.UnlinkedIDs {
			if u == id {
				t.Fatalf("did not expect %q counted as unlinked (code-free or project claim)", id)
			}
		}
	}
}

func TestStatus_UnlinkedCounting_DraftClaimsNeverCount(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{
		{ID: "widget.contract.draft", Module: "widget", Status: model.StatusDraft},
	}
	file := writeSourceFile(t, cfg, "a.go", "package widget")
	locked := []model.Claim{lockedClaim("widget.contract.locked", "widget")}
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
	r := &StatusReport{LinkedClaims: 2, Drifted: []DriftEntry{{}}, PartialCount: 1, UnlinkedCount: 3}
	want := "impl-links: 2 linked, 1 drifted, 1 partial, 3 unlinked locked claims"
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
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget")}
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

// ---------------------------------------------------------------------
// Step coverage and Coverage (issue #78: one step tag must not clear a
// whole claim, and a module with no artifact is a module of unlinked claims)
// ---------------------------------------------------------------------

func TestStepCoverage_CountsDistinctInRangeIndexes(t *testing.T) {
	covered, missing := StepCoverage(3, []int{2, 2, 0, 7})
	if covered != 1 || len(missing) != 2 || missing[0] != 1 || missing[1] != 3 {
		t.Fatalf("StepCoverage(3, [2 2 0 7]) = %d, %v; want 1, [1 3]", covered, missing)
	}
	if covered, missing := StepCoverage(0, []int{1}); covered != 0 || missing != nil {
		t.Fatalf("a claim with no steps must be trivially covered, got %d, %v", covered, missing)
	}
	if covered, missing := StepCoverage(2, []int{1, 2}); covered != 2 || missing != nil {
		t.Fatalf("full coverage must report nothing missing, got %d, %v", covered, missing)
	}
}

func TestStatus_ClaimTagOnSteppedClaim_IsPartialZeroOfN(t *testing.T) {
	cfg := testConfig(t, "widget")
	c := lockedClaim("widget.contract.main", "widget")
	c.Steps = []string{"alpha", "beta"}
	c.Layout = model.LayoutSteps
	claims := []model.Claim{c}
	file := writeSourceFile(t, cfg, "a.go", "package widget")
	// A whole-claim link (claim link / dossierx-claim) grounds the file for
	// drift but attests no step: the claim is linked, and 0 of 2 covered.
	if _, err := Set(claims, cfg, "widget", c.ID, file, ""); err != nil {
		t.Fatalf("Set: %v", err)
	}
	report, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if report.UnlinkedCount != 0 || report.LinkedClaims != 1 {
		t.Fatalf("a claim-tagged stepped claim is linked, not unlinked: %+v", report)
	}
	if report.PartialCount != 1 || len(report.Partial) != 1 {
		t.Fatalf("expected exactly one partial entry, got %+v", report.Partial)
	}
	p := report.Partial[0]
	if p.ClaimID != c.ID || p.Covered != 0 || p.Total != 2 || len(p.Missing) != 2 {
		t.Fatalf("expected 0 of 2 with steps 1 and 2 missing, got %+v", p)
	}
	if report.Incomplete() != 1 {
		t.Fatalf("Incomplete() must count the partial claim, got %d", report.Incomplete())
	}
}

func TestCoverage_NoArtifact_ListsEveryExpectedClaimAsUnlinked(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{
		lockedClaim("widget.contract.a", "widget"),
		lockedClaim("widget.contract.b", "widget"),
		codeFree(lockedClaim("widget.contract.ctx", "widget")),
	}
	draft := lockedClaim("widget.contract.d", "widget")
	draft.Status = model.StatusDraft
	claims = append(claims, draft)

	// Status keeps its silent-when-unused contract...
	if _, err := Status(claims, cfg, "widget"); !errors.Is(err, ErrNoArtifact) {
		t.Fatalf("Status on a module with no artifact must still wrap ErrNoArtifact, got %v", err)
	}
	// ...and Coverage evaluates the empty artifact instead.
	report, err := Coverage(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	if report.LinkedClaims != 0 || report.PartialCount != 0 {
		t.Fatalf("an empty artifact links nothing: %+v", report)
	}
	if report.UnlinkedCount != 2 || len(report.UnlinkedIDs) != 2 || report.UnlinkedIDs[0] != "widget.contract.a" || report.UnlinkedIDs[1] != "widget.contract.b" {
		t.Fatalf("expected the two locked claims, sorted, as unlinked; code-free and draft excluded: %+v", report.UnlinkedIDs)
	}
}

func TestExpects_EveryLockedModuleClaimUnlessCodeFree(t *testing.T) {
	c := lockedClaim("widget.contract.a", "widget")
	if !Expects(c) {
		t.Fatal("a locked module claim is expected to be linked")
	}
	if Expects(codeFree(c)) {
		t.Fatal("a claim declaring embodiment mode none is never expected to be linked")
	}
	compare := c
	compare.Embodiment = &model.Embodiment{Mode: model.EmbodimentModeCompare}
	if !Expects(compare) {
		t.Fatal("an embodiment in compare mode still expects a code link")
	}
	project := model.Claim{ID: "project.house-rule", Scope: model.ScopeProject, Status: model.StatusLocked}
	if Expects(project) {
		t.Fatal("a project claim is never expected to be linked")
	}
	c.Status = model.StatusDraft
	if Expects(c) {
		t.Fatal("a draft claim is never expected to be linked")
	}
}
