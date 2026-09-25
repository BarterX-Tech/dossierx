// Package manifest loads and validates the one required module
// manifest.yaml under claims_dir/<module>/manifest.yaml.
//
// A manifest is not a claim. provides is the module's export list: the
// contract-surface claim ids (facet "contract") other modules may pin.
// depends_on names contract ids of OTHER modules, and each must appear in
// that provider's provides. Neither list is a rests_on / readiness /
// catalog graph edge. Decided on Linear NIT-7, 2026-09-24.
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// FileName is the only legal basename at the required path.
const FileName = "manifest.yaml"

// isManifestFileName reports whether the file's base name is a module
// manifest (manifest.yaml or manifest.yml, any case). Deliberately NOT
// shared with internal/loader.IsManifestFileName (the two packages agree on
// the same predicate, defined twice): internal/loader has module-manifest
// awareness (internal/lint) as a dependent, and importing internal/loader
// from here to reuse its copy would close config -> manifest -> loader ->
// lint -> manifest into a cycle. See internal/loader.IsManifestFileName's
// own doc comment for the twin.
func isManifestFileName(name string) bool {
	base := name
	base = strings.ReplaceAll(base, "\\", "/")
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.EqualFold(base, "manifest.yaml") || strings.EqualFold(base, "manifest.yml")
}

// ContractFacet is the contract-surface facet the two lists align to
// (NIT-20 hard-locks facets; this package only reads the name).
const ContractFacet = "contract"

// MaxFileBytes is the hard file-size cap. check and claim lock refuse
// anything larger. It is a guard on the file, not the content budget:
// the budget is MaxSummaryRunes plus NIT-14's claims-per-module cap.
const MaxFileBytes = 4096

// MaxSummaryRunes is the hard summary cap in Unicode code points — the
// same unit NIT-8 chose for claim caps. The field is the single
// "why / start here" plus short neighbor/product usage — not a claim dump.
const MaxSummaryRunes = 280

// Manifest is the decoded module file.
type Manifest struct {
	Summary   string   `yaml:"summary" json:"summary"`
	Provides  []string `yaml:"provides" json:"provides"`
	DependsOn []string `yaml:"depends_on" json:"depends_on"`
}

// Finding is one module-manifest defect. Module is the module whose file
// is defective; "" marks a project-wide defect (a misplaced file, a walk
// error) that belongs to no single module.
type Finding struct {
	Module  string
	Message string
}

// RequiredRelPath is claims_dir-relative slash path for module.
func RequiredRelPath(module string) string {
	return path.Join(module, FileName)
}

// DraftCommand is the command an agent runs to draft a manifest. Every
// refusal that asks for authoring names it, so the recovery is never prose.
func DraftCommand(module string) string {
	return "dossierx manifest show " + module + " --isolation"
}

// StubYAML is what `claim new` writes when the module has no manifest yet.
// The summary is deliberately EMPTY: a stub must fail module-manifest until
// an agent drafts it from the module's claims. A stub that passed would be
// a placeholder nothing ever forces anyone to replace.
func StubYAML(module string) []byte {
	return []byte("# Module manifest for " + module + " — the durable why / start here.\n" +
		"# Draft summary, provides and depends_on from this module's claims:\n" +
		"#   " + DraftCommand(module) + "\n" +
		"# provides   = this module's contract-facet ids other modules may pin.\n" +
		"# depends_on = other modules' contract ids, each listed in that module's provides.\n" +
		"# Do not paste claim bodies. check and claim lock refuse this file until summary is written.\n" +
		"summary: \"\"\n" +
		"provides: []\n" +
		"depends_on: []\n")
}

// WriteStub writes StubYAML at RequiredRelPath under claimsDir.
func WriteStub(claimsDir, module string) error {
	return writeFile(claimsDir, module, StubYAML(module))
}

// MinimalYAML returns a valid empty-surface manifest. It exists for fixtures
// and tests that need a module to pass module-manifest without authoring
// one; the CLI never writes it (see StubYAML).
func MinimalYAML(module string) []byte {
	return []byte("summary: module " + module + " — fixture module context.\nprovides: []\ndepends_on: []\n")
}

// WriteMinimal writes MinimalYAML at RequiredRelPath under claimsDir.
// Test seeding only.
func WriteMinimal(claimsDir, module string) error {
	return writeFile(claimsDir, module, MinimalYAML(module))
}

func writeFile(claimsDir, module string, body []byte) error {
	rel := RequiredRelPath(module)
	dest := filepath.Join(claimsDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, body, 0o644)
}

// seedConfig is the subset of project.config.yaml SeedMinimalFromConfigYAML
// needs. Tests use this after writing a config so check/lock see a valid
// per-module manifest without every fixture listing one by hand.
type seedConfig struct {
	ClaimsDir string   `yaml:"claims_dir"`
	Modules   []string `yaml:"modules"`
}

// SeedMinimalFromConfigYAML writes a valid empty-surface manifest.yaml for
// every module listed in cfgYAML. projectRoot is the directory that contains
// project.config.yaml (claims_dir is resolved against it). Existing files
// are overwritten with the same stub; missing modules are a no-op.
// Test seeding only.
func SeedMinimalFromConfigYAML(projectRoot string, cfgYAML []byte) error {
	var c seedConfig
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

// Check validates one required manifest per cfg.Modules, then the
// cross-module rule: every depends_on id must be in its provider's
// provides. Findings are sorted by module so check output is stable.
func Check(claims []model.Claim, cfg *config.Config) []Finding {
	if cfg == nil {
		return nil
	}
	tree, extras, walkErr := loadTree(cfg)
	if walkErr != nil {
		return []Finding{{Module: "", Message: walkErr.Error()}}
	}

	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		byID[c.ID] = c
	}

	var findings []Finding
	parsed := make(map[string]Manifest, len(cfg.Modules))
	required := make(map[string]bool, len(cfg.Modules))
	for _, module := range cfg.Modules {
		rel := RequiredRelPath(module)
		required[rel] = true
		raw, ok := tree[rel]
		if !ok {
			findings = append(findings, Finding{
				Module:  module,
				Message: fmt.Sprintf("module %q is missing required %s (exactly one YAML manifest per module); draft it from: %s", module, rel, DraftCommand(module)),
			})
			continue
		}
		m, decodeFindings := decodeManifest(module, rel, raw)
		if len(decodeFindings) > 0 {
			findings = append(findings, decodeFindings...)
			continue
		}
		parsed[module] = m
		findings = append(findings, fieldFindings(module, rel, m, byID)...)
	}

	// The export rule. A provider whose own file is missing or undecodable
	// already carries its own finding; its dependants are not blamed twice.
	for _, module := range cfg.Modules {
		m, ok := parsed[module]
		if !ok {
			continue
		}
		rel := RequiredRelPath(module)
		for _, id := range m.DependsOn {
			c, ok := byID[id]
			if !ok || c.Module == "" || c.Module == module || c.Facet != ContractFacet {
				continue // fieldFindings reported it
			}
			pm, ok := parsed[c.Module]
			if !ok {
				continue
			}
			if !contains(pm.Provides, id) {
				findings = append(findings, Finding{
					Module: module,
					Message: fmt.Sprintf("%s depends_on names %s, which module %q does not list in provides; a module may only depend on what its provider exports",
						rel, id, c.Module),
				})
			}
		}
	}

	for _, rel := range extras {
		if required[rel] {
			continue
		}
		findings = append(findings, Finding{
			Module:  "",
			Message: fmt.Sprintf("%s is not a module manifest; the only legal path is claims_dir/<module>/%s", rel, FileName),
		})
	}
	return findings
}

func loadTree(cfg *config.Config) (tree map[string][]byte, extras []string, err error) {
	if cfg.ManifestTree != nil {
		return cfg.ManifestTree, extraManifests(cfg, cfg.ManifestTree), nil
	}

	tree = map[string][]byte{}
	if cfg.ClaimsDir == "" {
		return tree, nil, nil
	}
	info, statErr := os.Stat(cfg.ClaimsDir)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return tree, nil, nil
		}
		return nil, nil, fmt.Errorf("module-manifest: claims_dir %q: %w", cfg.ClaimsDir, statErr)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("module-manifest: claims_dir %q is not a directory", cfg.ClaimsDir)
	}

	walkErr := filepath.WalkDir(cfg.ClaimsDir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		if !isManifestFileName(d.Name()) {
			return nil
		}
		rel, relErr := filepath.Rel(cfg.ClaimsDir, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		raw, readErr := os.ReadFile(p)
		if readErr != nil {
			return fmt.Errorf("module-manifest: read %s: %w", rel, readErr)
		}
		tree[rel] = raw
		return nil
	})
	if walkErr != nil {
		return nil, nil, walkErr
	}
	return tree, extraManifests(cfg, tree), nil
}

func extraManifests(cfg *config.Config, tree map[string][]byte) []string {
	var extras []string
	for rel := range tree {
		if !isManifestFileName(rel) {
			continue
		}
		if isRequiredRel(cfg, rel) {
			continue
		}
		extras = append(extras, rel)
	}
	sort.Strings(extras)
	return extras
}

func isRequiredRel(cfg *config.Config, rel string) bool {
	for _, module := range cfg.Modules {
		if rel == RequiredRelPath(module) {
			return true
		}
	}
	return false
}

// ParseBytes decodes and validates one module manifest's own fields. It
// judges bytes only: a missing file and the cross-module export rule are
// Check's job, so a caller wanting the same verdict check gives must filter
// Check's findings (see Show) rather than call this alone.
func ParseBytes(module, rel string, raw []byte, byID map[string]model.Claim) (Manifest, []Finding) {
	m, findings := decodeManifest(module, rel, raw)
	if len(findings) > 0 {
		return Manifest{}, findings
	}
	return m, fieldFindings(module, rel, m, byID)
}

// decodeManifest turns bytes into a Manifest or exactly one finding: the
// size cap, a strict decode, or a second YAML document.
func decodeManifest(module, rel string, raw []byte) (Manifest, []Finding) {
	if len(raw) > MaxFileBytes {
		return Manifest{}, []Finding{{
			Module:  module,
			Message: fmt.Sprintf("%s is %d bytes; module manifests must be at most %d bytes", rel, len(raw), MaxFileBytes),
		}}
	}

	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, []Finding{{
			Module:  module,
			Message: fmt.Sprintf("%s is not a valid module manifest: %v", rel, err),
		}}
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err == nil || !isEOF(err) {
		return Manifest{}, []Finding{{
			Module:  module,
			Message: fmt.Sprintf("%s contains more than one YAML document; exactly one is required", rel),
		}}
	}
	return m, nil
}

// fieldFindings validates a decoded manifest's own fields against the
// claim corpus: the summary caps and the shape of both id lists.
func fieldFindings(module, rel string, m Manifest, byID map[string]model.Claim) []Finding {
	var findings []Finding
	summary := strings.TrimSpace(m.Summary)
	if summary == "" {
		findings = append(findings, Finding{
			Module: module,
			Message: fmt.Sprintf("%s has an empty summary (the module's why / start here); draft it from the module's claims: %s — do not paste claim bodies",
				rel, DraftCommand(module)),
		})
	} else if utf8.RuneCountInString(summary) > MaxSummaryRunes {
		findings = append(findings, Finding{
			Module: module,
			Message: fmt.Sprintf("%s summary is %d characters; must be at most %d (short why plus neighbor/product usage, not claim bodies)",
				rel, utf8.RuneCountInString(summary), MaxSummaryRunes),
		})
	}

	findings = append(findings, checkIDs(module, rel, "provides", m.Provides, byID, true)...)
	findings = append(findings, checkIDs(module, rel, "depends_on", m.DependsOn, byID, false)...)
	return findings
}

func isEOF(err error) bool {
	return errors.Is(err, io.EOF)
}

func checkIDs(module, rel, field string, ids []string, byID map[string]model.Claim, provides bool) []Finding {
	var findings []Finding
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" {
			findings = append(findings, Finding{
				Module:  module,
				Message: fmt.Sprintf("%s %s contains an empty id", rel, field),
			})
			continue
		}
		if seen[id] {
			findings = append(findings, Finding{
				Module:  module,
				Message: fmt.Sprintf("%s %s lists %s more than once", rel, field, id),
			})
			continue
		}
		seen[id] = true
		c, ok := byID[id]
		if !ok {
			findings = append(findings, Finding{
				Module:  module,
				Message: fmt.Sprintf("%s %s names unknown claim %s", rel, field, id),
			})
			continue
		}
		if c.Module == "" {
			findings = append(findings, Finding{
				Module: module,
				Message: fmt.Sprintf("%s %s names %s, which is not a module claim; project claims and constitution entries are rests_on targets, never manifest ids",
					rel, field, id),
			})
			continue
		}
		if c.Facet != ContractFacet {
			findings = append(findings, Finding{
				Module: module,
				Message: fmt.Sprintf("%s %s names %s, whose facet is %q; only %s-facet (contract-surface) ids are allowed",
					rel, field, id, c.Facet, ContractFacet),
			})
			continue
		}
		if provides && c.Module != module {
			findings = append(findings, Finding{
				Module:  module,
				Message: fmt.Sprintf("%s provides names %s, which belongs to module %q; provides is this module's own contract-surface ids", rel, id, c.Module),
			})
			continue
		}
		if !provides && c.Module == module {
			findings = append(findings, Finding{
				Module:  module,
				Message: fmt.Sprintf("%s depends_on names %s from this module; depends_on lists other modules' contract ids only", rel, id),
			})
		}
	}
	return findings
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
