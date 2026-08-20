package markdown

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// markdown_claim_body_golden_test.go is the golden corpus for the ONE construct
// that renders differently depending on which entry point a caller reached, and
// it is shaped around that difference rather than around the construct.
//
// Every fixture in testdata/markdown-claim-body-cases carries TWO goldens:
//
//	<name>.claim.golden.html    RenderClaimBody — the claim-body surface
//	<name>.comment.golden.html  Render          — every other surface, which is
//	                                            what a comment body reaches
//
// The pair IS the test. A reviewer reading a diff of these two files is reading
// the exact difference the opt-in makes, on the same source bytes, which is the
// thing gate 0 decided and the thing a later change could silently undo. A
// single golden per fixture would pin the claim surface and say nothing about
// the comment one, and it is the comment one that matters.
//
// testdata/markdown-cases (the 129-fixture corpus) stays Render-only and keeps
// its own harness. These fixtures are not added there because there is nothing
// there to hold the second half of the pair.

// claimBodyGoldenPrefix is the AssetPrefix the claim goldens are generated with.
// It is a fixed literal rather than the production shape so the goldens record
// the RENDERER's contract — "the prefix is prepended to an accepted path and to
// nothing else" — and do not have to be regenerated if internal/render/
// components ever changes how it spells a claim's asset route.
const claimBodyGoldenPrefix AssetPrefix = "/claim-assets/widget.contract.demo/"

// claimBodyCasesDir locates testdata/markdown-claim-body-cases the same way
// markdownCasesDir locates its corpus.
func claimBodyCasesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "markdown-claim-body-cases")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("markdown-claim-body-cases dir not found at %q: %v", dir, err)
	}
	return dir
}

// claimBodyGoldenNames returns the two golden filenames for one fixture.
func claimBodyGoldenNames(yamlFile string) (claim, comment string) {
	base := strings.TrimSuffix(yamlFile, ".yaml")
	return base + ".claim.golden.html", base + ".comment.golden.html"
}

// TestClaimBodyGoldenFileCompleteness keeps the corpus and its goldens in step,
// in BOTH directions and for BOTH surfaces: a fixture with only one golden is
// the exact defect this file exists to prevent, because the missing one is
// always the comment surface.
func TestClaimBodyGoldenFileCompleteness(t *testing.T) {
	dir := claimBodyCasesDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read markdown-claim-body-cases: %v", err)
	}

	yamlFiles := map[string]bool{}
	goldens := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch {
		case strings.HasSuffix(e.Name(), ".yaml"):
			yamlFiles[e.Name()] = true
		case strings.HasSuffix(e.Name(), ".golden.html"):
			goldens[e.Name()] = true
		}
	}
	if len(yamlFiles) == 0 {
		t.Fatal("no YAML fixtures found in markdown-claim-body-cases")
	}

	expected := map[string]bool{}
	for y := range yamlFiles {
		claim, comment := claimBodyGoldenNames(y)
		expected[claim], expected[comment] = true, true
		for _, want := range []string{claim, comment} {
			if !goldens[want] {
				t.Errorf("fixture %s has no %s", y, want)
			}
		}
	}
	for g := range goldens {
		if !expected[g] {
			t.Errorf("orphaned golden %s (no corresponding fixture)", g)
		}
	}
	t.Logf("%d fixtures, %d goldens", len(yamlFiles), len(goldens))
}

// TestClaimBodyGoldenRenderConsistency is the regression test. Every fixture is
// rendered through BOTH entry points and both results must match their goldens.
//
// It also asserts, on every fixture and independently of the goldens, the two
// invariants that must hold no matter what a fixture says: Render never emits an
// image, and RenderClaimBody never emits one whose src is not the prefix
// followed by a path ClaimBodyImages also reports. The goldens can be
// regenerated; these cannot.
func TestClaimBodyGoldenRenderConsistency(t *testing.T) {
	if *regenerateGoldens {
		t.Run("regenerate", func(t *testing.T) { regenerateClaimBodyGoldens(t) })
		return
	}

	dir := claimBodyCasesDir(t)
	for _, yamlFile := range claimBodyFixtures(t, dir) {
		yamlFile := yamlFile
		t.Run(yamlFile, func(t *testing.T) {
			claim := loadClaim(t, dir, yamlFile)
			if claim.Body == "" {
				t.Fatalf("%s: empty body", yamlFile)
			}
			claimName, commentName := claimBodyGoldenNames(yamlFile)

			claimOut := string(RenderClaimBody(claim.Body, claimBodyGoldenPrefix, Citations{}))
			commentOut := string(Render(claim.Body))

			assertGoldenMatch(t, dir, claimName, claimOut)
			assertGoldenMatch(t, dir, commentName, commentOut)

			// Invariant 1: the default entry point has no image capability.
			if strings.Contains(commentOut, "<img") {
				t.Errorf("%s: Render emitted an image:\n%s", yamlFile, commentOut)
			}
			// Invariant 2: every src the claim surface emits is the prefix
			// plus a path ClaimBodyImages independently reports, so serve's
			// allowlist can never be missing one.
			reported := ClaimBodyImages(claim.Body)
			if got := strings.Count(claimOut, "<img"); got != len(reported) {
				t.Errorf("%s: rendered %d images but ClaimBodyImages reported %d (%v)",
					yamlFile, got, len(reported), reported)
			}
			for _, rel := range reported {
				want := `src="` + string(claimBodyGoldenPrefix) + rel + `"`
				if !strings.Contains(claimOut, want) {
					t.Errorf("%s: ClaimBodyImages reported %q but the render has no %s",
						yamlFile, rel, want)
				}
			}
			assertTagBalance(t, claimOut)
			assertTagBalance(t, commentOut)
		})
	}
}

// claimBodyFixtures lists the .yaml fixtures in dir.
func claimBodyFixtures(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read markdown-claim-body-cases: %v", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			out = append(out, e.Name())
		}
	}
	if len(out) == 0 {
		t.Fatal("no YAML fixtures found")
	}
	return out
}

// assertGoldenMatch compares one rendered result against one golden file, using
// the same "comment header, then the HTML" format markdown-cases uses.
func assertGoldenMatch(t *testing.T, dir, goldenFile, got string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, goldenFile))
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenFile, err)
	}
	content := string(data)
	idx := strings.Index(content, " -->\n")
	if idx == -1 {
		t.Fatalf("%s: invalid golden format (missing comment header)", goldenFile)
	}
	want := strings.TrimSuffix(content[idx+5:], "\n")
	if got != want {
		t.Errorf("render mismatch for %s\nExpected:\n%s\n\nGot:\n%s", goldenFile, want, got)
	}
}

func regenerateClaimBodyGoldens(t *testing.T) {
	dir := claimBodyCasesDir(t)
	for _, yamlFile := range claimBodyFixtures(t, dir) {
		claim := loadClaim(t, dir, yamlFile)
		if claim.Body == "" {
			t.Logf("skipping %s: empty body", yamlFile)
			continue
		}
		claimName, commentName := claimBodyGoldenNames(yamlFile)
		writeGolden(t, dir, claimName, yamlFile, string(RenderClaimBody(claim.Body, claimBodyGoldenPrefix, Citations{})))
		writeGolden(t, dir, commentName, yamlFile, string(Render(claim.Body)))
		t.Logf("regenerated: %s, %s", claimName, commentName)
	}
}

func writeGolden(t *testing.T, dir, goldenFile, yamlFile, rendered string) {
	t.Helper()
	content := "<!-- Generated golden file for " + yamlFile + " -->\n" + rendered + "\n"
	if err := os.WriteFile(filepath.Join(dir, goldenFile), []byte(content), 0o644); err != nil {
		t.Fatalf("write golden %s: %v", goldenFile, err)
	}
}
