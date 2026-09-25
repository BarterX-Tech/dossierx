package manifest

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ViewerModule is the viewer's read-only projection of one module's
// manifest.yaml (NIT-19): one small file per module, no traversal and no
// claim-graph walk.
//
// Findings is check's module-manifest verdict for this module, filtered the
// way Show filters it (the module's own findings plus the project-wide ones
// no module owns), so the Manifest tab can never read healthy where check or
// manifest show refuses. When Findings is non-empty, Manifest and Raw are
// zero: a broken file is never soft-rendered as healthy.
type ViewerModule struct {
	Module   string
	Path     string
	Manifest Manifest
	// Raw is the file's text, at most MaxFileBytes: a larger file is a
	// finding and its bytes are never carried.
	Raw      string
	Findings []Finding
	// Command is DraftCommand(Module), the recovery every refusal names.
	Command string
}

// Healthy reports whether the manifest passed module-manifest.
func (v ViewerModule) Healthy() bool { return len(v.Findings) == 0 }

// Viewer returns one ViewerModule per cfg.Modules entry. It reads the
// manifest tree once and runs Check's own rules over it, so the findings are
// byte-for-byte the ones the module-manifest lint reports. A nil cfg or one
// with no modules returns nil.
func Viewer(claims []model.Claim, cfg *config.Config) map[string]ViewerModule {
	if cfg == nil || len(cfg.Modules) == 0 {
		return nil
	}
	var findings []Finding
	tree, extras, walkErr := loadTree(cfg)
	if walkErr != nil {
		findings = []Finding{{Module: "", Message: walkErr.Error()}}
	} else {
		findings = checkTree(claims, cfg, tree, extras)
	}

	out := make(map[string]ViewerModule, len(cfg.Modules))
	for _, module := range cfg.Modules {
		rel := RequiredRelPath(module)
		v := ViewerModule{Module: module, Path: rel, Command: DraftCommand(module)}
		for _, f := range findings {
			if f.Module == module || f.Module == "" {
				v.Findings = append(v.Findings, f)
			}
		}
		if len(v.Findings) == 0 {
			raw := tree[rel]
			v.Manifest, _ = decodeManifest(module, rel, raw)
			v.Raw = string(raw)
		}
		out[module] = v
	}
	return out
}
