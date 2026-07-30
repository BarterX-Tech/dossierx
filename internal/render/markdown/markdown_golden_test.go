package markdown

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regenerateGoldens is a flag to regenerate golden files on go test
var regenerateGoldens = flag.Bool("regenerate-goldens", false, "regenerate markdown golden files")

// TestMarkdownGoldenFileCompleteness verifies that every YAML fixture has a
// corresponding .golden.html file and vice versa. This prevents accidental
// mismatches where a fixture is added without a golden file, or vice versa.
//
// To regenerate all golden files, run:
//
//	go test ./... -regenerate-goldens
func TestMarkdownGoldenFileCompleteness(t *testing.T) {
	dir := markdownCasesDir(t)

	// Read all files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read markdown-cases dir: %v", err)
	}

	yamlFiles := make(map[string]bool)
	goldenFiles := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") {
			yamlFiles[name] = true
		} else if strings.HasSuffix(name, ".golden.html") {
			goldenFiles[name] = true
		}
	}

	if len(yamlFiles) == 0 {
		t.Fatal("no YAML fixtures found in markdown-cases directory")
	}

	// Check that every YAML has a corresponding golden
	for yamlFile := range yamlFiles {
		expectedGolden := strings.TrimSuffix(yamlFile, ".yaml") + ".golden.html"
		if !goldenFiles[expectedGolden] {
			t.Errorf("fixture %s has no corresponding golden file %s", yamlFile, expectedGolden)
		}
	}

	// Check that every golden has a corresponding YAML (no orphaned goldens)
	for goldenFile := range goldenFiles {
		expectedYAML := strings.TrimSuffix(goldenFile, ".golden.html") + ".yaml"
		if !yamlFiles[expectedYAML] {
			t.Errorf("orphaned golden file %s (no corresponding YAML %s)", goldenFile, expectedYAML)
		}
	}

	t.Logf("Found %d YAML fixtures and %d golden files", len(yamlFiles), len(goldenFiles))
}

// TestMarkdownGoldenRenderConsistency verifies that re-running Render on each
// fixture produces output matching its corresponding .golden.html file. This
// is the regression test: any change to markdown.Render's output will cause
// a mismatch, which must be justified in the commit message.
//
// To update golden files after intentional changes to markdown.Render,
// regenerate them with: go test ./... -regenerate-goldens
func TestMarkdownGoldenRenderConsistency(t *testing.T) {
	if *regenerateGoldens {
		t.Run("regenerate", func(t *testing.T) {
			testRegenerateGoldens(t)
		})
		return
	}

	dir := markdownCasesDir(t)

	// Read all YAML files
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read markdown-cases dir: %v", err)
	}

	var yamlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			yamlFiles = append(yamlFiles, entry.Name())
		}
	}

	if len(yamlFiles) == 0 {
		t.Fatal("no YAML fixtures found")
	}

	for _, yamlFile := range yamlFiles {
		yamlFile := yamlFile
		t.Run(yamlFile, func(t *testing.T) {
			claim := loadClaim(t, dir, yamlFile)
			if claim.Body == "" {
				t.Fatalf("%s: empty body", yamlFile)
			}

			// Render the body
			rendered := string(Render(claim.Body))

			// Load and verify the golden file
			goldenFile := strings.TrimSuffix(yamlFile, ".yaml") + ".golden.html"
			goldenPath := filepath.Join(dir, goldenFile)

			goldenData, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v", goldenFile, err)
			}

			goldenContent := string(goldenData)

			// The golden file contains a comment header followed by the rendered HTML
			// Extract just the HTML portion (everything after the comment)
			idx := strings.Index(goldenContent, " -->\n")
			if idx == -1 {
				t.Fatalf("%s: invalid golden file format (missing comment)", goldenFile)
			}
			expectedHTML := goldenContent[idx+5:] // Skip " -->\n"
			expectedHTML = strings.TrimSuffix(expectedHTML, "\n")

			if rendered != expectedHTML {
				t.Errorf("render output mismatch for %s\nExpected:\n%s\n\nGot:\n%s",
					yamlFile, expectedHTML, rendered)
			}
		})
	}
}

// TestMarkdownGoldenRenderInline tests RenderInline where applicable.
// Currently, all fixtures test Render. This test is a placeholder for
// inline-specific fixture testing if needed in the future.
func TestMarkdownGoldenRenderInline(t *testing.T) {
	// Inline-specific cases (if any) would go here.
	// For now, this is a placeholder to document the capability.
	t.Skip("inline-only fixtures not yet added")
}

func testRegenerateGoldens(t *testing.T) {
	dir := markdownCasesDir(t)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read markdown-cases dir: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		yamlFile := entry.Name()
		claim := loadClaim(t, dir, yamlFile)
		if claim.Body == "" {
			t.Logf("Skipping %s: empty body", yamlFile)
			continue
		}

		// Render the body
		rendered := string(Render(claim.Body))

		// Create golden file content with comment header
		goldenFile := strings.TrimSuffix(yamlFile, ".yaml") + ".golden.html"
		goldenPath := filepath.Join(dir, goldenFile)

		content := "<!-- Generated golden file for " + yamlFile + " -->\n" + rendered + "\n"

		if err := os.WriteFile(goldenPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write golden file %s: %v", goldenFile, err)
		}

		t.Logf("Regenerated: %s", goldenFile)
		count++
	}

	t.Logf("Regenerated %d golden files", count)
}
