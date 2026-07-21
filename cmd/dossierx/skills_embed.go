// skills_embed.go wires the embedded skills/ directory into the CLI as
// "dossierx skills export <dir>": walks every embedded skill file and
// writes it to the target directory, creating parent directories as
// needed and overwriting any existing files. The construction pattern
// (newXCmd() *cobra.Command, added to newRootCmd()'s AddCommand list) is
// identical to every other command in this package.
//
// The actual //go:embed directive lives in
// github.com/BarterX-Tech/dossierx/skills (see skills/embed.go), not
// here: go:embed patterns must not contain ".." path elements, so a file
// under cmd/dossierx/ cannot embed the repo-root skills/ directory
// directly. This file only imports that package's embed.FS and walks it.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	dxskills "github.com/BarterX-Tech/dossierx/skills"
)

// newSkillsCmd is the "dossierx skills" command group.
func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Work with the embedded Claude Code skill files (dossierx-claims, dossierx-build-order, dossierx-code-links)",
	}
	cmd.AddCommand(newSkillsExportCmd())
	return cmd
}

func newSkillsExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <dir>",
		Short: "Write every embedded skill file to <dir>, creating parent dirs as needed and overwriting existing files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetDir := args[0]
			written, err := exportSkills(dxskills.FS, targetDir)
			if err != nil {
				return fmt.Errorf("skills export: %w", err)
			}
			out := cmd.OutOrStdout()
			for _, p := range written {
				fmt.Fprintf(out, "skills export: wrote %s\n", p)
			}
			fmt.Fprintf(out, "skills export: wrote %d file(s) to %s\n", len(written), targetDir)
			return nil
		},
	}
}

// exportSkills walks every file in embedded and writes it under targetDir,
// preserving the embedded path layout (e.g. dossierx-claims/SKILL.md).
// Parent directories are created as needed; existing files are
// overwritten. Returns the list of paths written, in walk order.
func exportSkills(embedded fs.FS, targetDir string) ([]string, error) {
	var written []string
	err := fs.WalkDir(embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(embedded, path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		outPath := filepath.Join(targetDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", outPath, err)
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		written = append(written, outPath)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return written, nil
}
