package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
)

func withManifests(cfg *config.Config) *config.Config {
	if cfg == nil {
		return nil
	}
	tree := make(map[string][]byte, len(cfg.Modules))
	for _, module := range cfg.Modules {
		tree[manifest.RequiredRelPath(module)] = manifest.MinimalYAML(module)
	}
	cfg.ManifestTree = tree
	return cfg
}
