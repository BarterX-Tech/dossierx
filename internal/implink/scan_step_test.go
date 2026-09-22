package implink

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func lockedStepsClaim(id, module string, steps []string) model.Claim {
	c := lockedClaim(id, module, model.BuildRoleBehavior)
	c.Steps = steps
	c.Layout = model.LayoutSteps
	return c
}

func TestScan_ValidStepTag_ReconcilesALink(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	steps := []string{"alpha", "beta"}
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", steps)}
	want := StepContentHash("beta")
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #2 "+want+"\nfunc DoBeta() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("expected no scan errors, got %+v", report.Errors)
	}
	if len(report.Matches) != 1 {
		t.Fatalf("expected 1 match, got %+v", report.Matches)
	}
	if report.Matches[0].Step != 2 || report.Matches[0].StepHash != want {
		t.Fatalf("unexpected step match: %+v", report.Matches[0])
	}
	artifact, err := LoadArtifact(ArtifactPath(cfg, "widget"))
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if len(artifact.Links) != 1 || len(artifact.Links[0].Files) != 1 {
		t.Fatalf("expected one linked file, got %+v", artifact.Links)
	}
	got := artifact.Links[0].Files[0]
	if got.Step != 2 || got.StepHash != want || got.File != "src/main.go" {
		t.Fatalf("unexpected artifact row: %+v", got)
	}
}

func TestScan_StepTagAndClaimTag_AreAdditive(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	steps := []string{"only"}
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", steps)}
	want := StepContentHash("only")
	writeScanFile(t, srcDir, "a.go", "// dossierx-claim: widget.contract.main\nfunc Whole() {}\n")
	writeScanFile(t, srcDir, "b.go", "// dossierx-step: widget.contract.main #1 "+want+"\nfunc StepOne() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 0 || len(report.Matches) != 2 {
		t.Fatalf("expected 2 matches 0 errors, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
	artifact, err := LoadArtifact(ArtifactPath(cfg, "widget"))
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if len(artifact.Links) != 1 || len(artifact.Links[0].Files) != 2 {
		t.Fatalf("expected two file rows on one claim, got %+v", artifact.Links)
	}
}

func TestScan_BareStepTag_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"a"})}
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Matches) != 0 || len(report.Errors) != 1 {
		t.Fatalf("expected 1 grammar error, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
	if report.Errors[0].Marker != "dossierx-step" || !strings.Contains(report.Errors[0].Message, "tag must be") {
		t.Fatalf("unexpected error: %+v", report.Errors[0])
	}
}

func TestScan_StepHashPrefix_ReconcilesAndRecordsFullDigest(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	steps := []string{"hello\n\tworld  "}
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", steps)}
	want := StepContentHash(steps[0])
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+want[:StepHashPreferredPrefixLen]+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 0 || len(report.Matches) != 1 {
		t.Fatalf("expected prefix to reconcile, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
	if report.Matches[0].StepHash != want {
		t.Fatalf("artifact must record the full digest, got %s want %s", report.Matches[0].StepHash, want)
	}
	artifact, err := LoadArtifact(ArtifactPath(cfg, "widget"))
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if artifact.Links[0].Files[0].StepHash != want {
		t.Fatalf("expected full step_hash on disk, got %+v", artifact.Links[0].Files[0])
	}
}

func TestScan_ShortStepHash_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"hello world"})}
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+StepContentHash("hello world")[:7]+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Matches) != 0 || len(report.Errors) != 1 {
		t.Fatalf("expected 7-hex grammar error, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
	if !strings.Contains(report.Errors[0].Message, "tag must be") {
		t.Fatalf("unexpected error: %+v", report.Errors[0])
	}
}

func TestScan_RawUnnormalisedHash_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	raw := "hello\nworld"
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{raw})}
	sum := sha256.Sum256([]byte(raw))
	old := hex.EncodeToString(sum[:])
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+old+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Matches) != 0 || len(report.Errors) != 1 {
		t.Fatalf("expected old raw hash to mismatch, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
	if !strings.Contains(report.Errors[0].Message, "hash mismatch") {
		t.Fatalf("unexpected error: %+v", report.Errors[0])
	}
}

func TestScan_StepHashMismatch_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"real"})}
	bad := strings.Repeat("0", 64)
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+bad+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Matches) != 0 || len(report.Errors) != 1 {
		t.Fatalf("expected hash mismatch, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
	if !strings.Contains(report.Errors[0].Message, "hash mismatch") {
		t.Fatalf("unexpected error: %+v", report.Errors[0])
	}
}

func TestScan_StepOutOfRange_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"only"})}
	want := StepContentHash("only")
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #9 "+want+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0].Message, "out of range") {
		t.Fatalf("expected out of range, got %+v", report.Errors)
	}
}

func TestScan_StepOnClaimWithNoSteps_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	want := StepContentHash("nope")
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+want+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 1 || report.Errors[0].Message != "claim has no steps" {
		t.Fatalf("expected no-steps error, got %+v", report.Errors)
	}
}

func TestScan_UnknownStepClaim_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"a"})}
	want := StepContentHash("a")
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.mian #1 "+want+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 1 || report.Errors[0].ClaimID != "widget.contract.mian" {
		t.Fatalf("expected unknown id, got %+v", report.Errors)
	}
}

func TestScan_DraftStepClaim_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{{
		ID: "widget.contract.main", Module: "widget", Status: model.StatusDraft,
		Steps: []string{"a"},
	}}
	want := StepContentHash("a")
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+want+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Matches) != 0 || len(report.Errors) != 1 {
		t.Fatalf("expected draft error, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
	if !strings.Contains(report.Errors[0].Message, "not locked") {
		t.Fatalf("unexpected error: %+v", report.Errors[0])
	}
}

func TestScan_ZeroStepIndex_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"a"})}
	want := StepContentHash("a")
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #0 "+want+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0].Message, "out of range") {
		t.Fatalf("expected #0 out of range, got %+v", report.Errors)
	}
}

func TestScan_TwoStepTagsOnOneLine_BothReconcile(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"a", "b"})}
	h1 := StepContentHash("a")
	h2 := StepContentHash("b")
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+h1+" dossierx-step: widget.contract.main #2 "+h2+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 0 || len(report.Matches) != 2 {
		t.Fatalf("expected 2 matches, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
}

func TestScan_ClaimAndStepOnOneLine_AreAdditive(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"a"})}
	h1 := StepContentHash("a")
	writeScanFile(t, srcDir, "main.go", "// dossierx-claim: widget.contract.main dossierx-step: widget.contract.main #1 "+h1+"\nfunc Foo() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 0 || len(report.Matches) != 2 {
		t.Fatalf("expected claim+step matches, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
}

func TestStatus_StepTagClearsUnlinked(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"alpha"})}
	want := StepContentHash("alpha")
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+want+"\nfunc Foo() {}\n")
	if _, err := Scan(claims, cfg); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	st, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.UnlinkedCount != 0 || st.LinkedClaims != 1 {
		t.Fatalf("step tag should count as linked, got %+v", st)
	}
}

// One tag on a two-step claim is NOT a linked claim. The fixture in
// TestStatus_StepTagClearsUnlinked has a single step, so it could never tell
// "any step tagged" from "every step tagged"; this one can, and it is the
// shape issue #78 names as the false green: a claim that looks linked in the
// unlinked count while most of its steps have no code behind them.
func TestStatus_OneOfTwoStepTags_IsPartial(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedStepsClaim("widget.contract.main", "widget", []string{"alpha", "beta"})}
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+StepContentHash("alpha")+"\nfunc Foo() {}\n")
	if _, err := Scan(claims, cfg); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	st, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.UnlinkedCount != 0 || st.LinkedClaims != 1 {
		t.Fatalf("one step tag still links the claim, got %+v", st)
	}
	if st.PartialCount != 1 || len(st.Partial) != 1 || st.Partial[0].Covered != 1 || st.Partial[0].Total != 2 || len(st.Partial[0].Missing) != 1 || st.Partial[0].Missing[0] != 2 {
		t.Fatalf("expected partial 1 of 2 missing step 2, got %+v", st.Partial)
	}

	// Tagging the second step completes it: the same file, one more tag.
	writeScanFile(t, srcDir, "main.go", "// dossierx-step: widget.contract.main #1 "+StepContentHash("alpha")+"\n// dossierx-step: widget.contract.main #2 "+StepContentHash("beta")+"\nfunc Foo() {}\n")
	if _, err := Scan(claims, cfg); err != nil {
		t.Fatalf("Scan (second): %v", err)
	}
	st, err = Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status (second): %v", err)
	}
	if st.PartialCount != 0 || st.Incomplete() != 0 {
		t.Fatalf("both steps tagged must be complete, got %+v", st)
	}
}
