// Command generate-goldens creates .golden.html files for each fixture in testdata/markdown-cases/.
// Usage: go run ./internal/render/markdown/generate-goldens/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
	"gopkg.in/yaml.v3"
)

type claimYAML struct {
	ID     string `yaml:"id"`
	Layout string `yaml:"layout"`
	Body   string `yaml:"body"`
}

func main() {
	flag.Parse()

	// Find the testdata/markdown-cases directory
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding repo root: %v\n", err)
		os.Exit(1)
	}

	casesDir := filepath.Join(repoRoot, "testdata", "markdown-cases")
	if info, err := os.Stat(casesDir); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "markdown-cases dir not found at %q\n", casesDir)
		os.Exit(1)
	}

	entries, err := os.ReadDir(casesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading cases directory: %v\n", err)
		os.Exit(1)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			if err := processFixture(casesDir, entry.Name()); err != nil {
				fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", entry.Name(), err)
				os.Exit(1)
			}
			count++
		}
	}

	fmt.Printf("Generated %d golden files\n", count)
}

func processFixture(casesDir, filename string) error {
	yamlPath := filepath.Join(casesDir, filename)
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}

	var claim claimYAML
	if err := yaml.Unmarshal(data, &claim); err != nil {
		return err
	}

	if claim.Body == "" {
		return fmt.Errorf("%s: empty body", filename)
	}

	// Generate golden filename (replace .yaml with .golden.html)
	baseName := strings.TrimSuffix(filename, ".yaml")
	goldenPath := filepath.Join(casesDir, baseName+".golden.html")

	// Render the body
	rendered := markdown.Render(claim.Body)

	// Write golden file
	content := fmt.Sprintf("<!-- Generated golden file for %s -->\n%s\n", filename, rendered)
	if err := os.WriteFile(goldenPath, []byte(content), 0o644); err != nil {
		return err
	}

	fmt.Printf("Generated: %s\n", goldenPath)
	return nil
}

// findRepoRoot walks up from the current directory to find the directory containing go.mod
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("could not find go.mod")
		}
		cwd = parent
	}
}
