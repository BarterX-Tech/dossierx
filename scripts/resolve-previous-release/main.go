package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/releasebaseline"
)

func main() {
	envFile := flag.String("github-env", "", "append resolved values to this GitHub Actions environment file")
	flag.Parse()
	eventRef := strings.TrimSpace(os.Getenv("GITHUB_REF"))
	if !strings.HasPrefix(eventRef, "refs/tags/") {
		eventRef = ""
	}
	result, err := releasebaseline.Resolve(releasebaseline.Options{
		RepoDir:     ".",
		OverrideTag: strings.TrimSpace(os.Getenv("DOSSIERX_PREV_RELEASE_TAG")),
		EventRef:    eventRef,
		EventCommit: strings.TrimSpace(os.Getenv("GITHUB_SHA")),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve previous release: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("current=%s %s\nbaseline=%s %s (%s)\n", result.CurrentVersion, result.CurrentCommit, result.BaselineTag, result.BaselineVersion, result.BaselineCommit)
	if *envFile == "" {
		return
	}
	if err := appendGitHubEnv(*envFile, []string{
		"DOSSIERX_CURRENT_VERSION=" + result.CurrentVersion,
		"DOSSIERX_CURRENT_COMMIT=" + result.CurrentCommit,
		"DOSSIERX_PREV_RELEASE_TAG=" + result.BaselineTag,
		"DOSSIERX_PREV_RELEASE_COMMIT=" + result.BaselineCommit,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func appendGitHubEnv(path string, lines []string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open github env: %w", err)
	}
	defer f.Close()
	for _, line := range lines {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("write github env: %w", err)
		}
	}
	return nil
}
