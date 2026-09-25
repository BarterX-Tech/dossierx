// Lint module-manifest requires exactly one capped YAML
// claims_dir/<module>/manifest.yaml per configured module. Missing,
// oversize, malformed, misplaced, or non-contract-surface
// provides/depends_on refuse check and claim lock.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ModuleManifestLintName is the finding's LintName. Its ClaimID is the
// MODULE whose file is defective ("" for a project-wide defect), never a
// claim id — lock-path scoping keys off this name to block every claim of
// that module (cmd/dossierx/lock_batch.go, internal/lock/policy.go).
const ModuleManifestLintName = "module-manifest"

func init() {
	Registry = append(Registry, moduleManifestLint{})
}

type moduleManifestLint struct{}

func (moduleManifestLint) Name() string { return ModuleManifestLintName }

func (moduleManifestLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	var findings []Finding
	for _, f := range manifest.Check(claims, cfg) {
		findings = append(findings, Finding{
			LintName: ModuleManifestLintName,
			ClaimID:  f.Module,
			Severity: SeverityError,
			Message:  f.Message,
		})
	}
	return findings
}
