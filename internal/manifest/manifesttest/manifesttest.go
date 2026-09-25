// Package manifesttest seeds valid module manifests for fixtures and tests
// that need a module to pass module-manifest without authoring one. The CLI
// never writes these (it writes manifest.StubYAML), so they live here rather
// than in internal/manifest's production surface.
package manifesttest

import (
	"os"
	"path/filepath"

	"github.com/BarterX-Tech/dossierx/internal/manifest"
	"gopkg.in/yaml.v3"
)

// MinimalYAML returns a valid empty-surface manifest for module.
func MinimalYAML(module string) []byte {
	return []byte("summary: module " + module + " — fixture module context.\nprovides: []\ndepends_on: []\n")
}

// WriteMinimal writes MinimalYAML at manifest.RequiredRelPath under claimsDir.
func WriteMinimal(claimsDir, module string) error {
	dest := filepath.Join(claimsDir, filepath.FromSlash(manifest.RequiredRelPath(module)))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, MinimalYAML(module), 0o644)
}

// SeedMinimalFromConfigYAML writes a valid empty-surface manifest.yaml for
// every module listed in cfgYAML. projectRoot is the directory that contains
// project.config.yaml (claims_dir is resolved against it). Existing files
// are overwritten with the same stub.
func SeedMinimalFromConfigYAML(projectRoot string, cfgYAML []byte) error {
	var c struct {
		ClaimsDir string   `yaml:"claims_dir"`
		Modules   []string `yaml:"modules"`
	}
	if err := yaml.Unmarshal(cfgYAML, &c); err != nil {
		return err
	}
	claimsDir := c.ClaimsDir
	if claimsDir == "" {
		claimsDir = "claims"
	}
	if !filepath.IsAbs(claimsDir) {
		claimsDir = filepath.Join(projectRoot, claimsDir)
	}
	for _, module := range c.Modules {
		if module == "" {
			continue
		}
		if err := WriteMinimal(claimsDir, module); err != nil {
			return err
		}
	}
	return nil
}
