// build_role_required.go implements the "build-role-required-for-locked"
// lint: model.Claim.BuildRole is optional while a claim is draft (a human
// may not have decided yet where a claim sits in its module's build
// sequence), but internal/buildorder can only compute a module's build
// order once every claim in it carries a valid BuildRole — so this lint
// enforces that requirement at the one lifecycle point that matters:
// locking. A claim that locks without ever setting build_role would
// otherwise silently defeat "docs build-order propose"'s completeness gate
// (it would look "fully locked" yet be unplaceable), so this is caught here
// instead, the same way governed-required catches a locked claim missing
// governed_by.
//
// The "locked but no build_role" check is deliberately scoped to modules
// that have already adopted build_role somewhere (see hasAdoptedBuildRole):
// a project that has never set build_role on any claim in a module hasn't
// opted into this feature at all, and per this engine's central invariant
// (nothing here may change existing lock/lint behavior for a project that
// doesn't use a feature it never touched), locking a claim in that module
// must keep behaving exactly as it did before this lint existed. Only once
// a module has at least one claim carrying a build_role does "every locked
// claim in it needs one too" start being enforced — which is also exactly
// when internal/buildorder's completeness gate would actually be run
// against that module for the first time.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, BuildRoleRequiredLint{})
}

// BuildRoleRequiredLint reports two distinct problems with a claim's
// BuildRole field:
//
//  1. a locked claim with no BuildRole set at all, in a module that has
//     otherwise adopted build_role (see hasAdoptedBuildRole) — build_role
//     is required once a claim locks, per this file's package doc comment,
//     but only once its module has actually started using the field.
//  2. any claim (draft or locked, any module) whose BuildRole is non-empty
//     but not one of the six values model.BuildRole defines — a typo'd or
//     stale value would otherwise silently fall through
//     internal/buildorder's phase lookup rather than failing loudly at
//     lint time, where a human is actually looking. This check is never
//     adoption-gated: a claim that sets build_role at all has, by
//     definition, already opted into needing it to be valid.
type BuildRoleRequiredLint struct{}

// Name returns this lint's rule name.
func (BuildRoleRequiredLint) Name() string { return "build-role-required-for-locked" }

// validBuildRoles is the fixed set of BuildRole values model.BuildRole
// defines. It is checked here (rather than delegating to some validation
// method on model.BuildRole itself) because Finding.Message needs to name
// the offending value, and lint is where every other "is this enum value
// legal" check in this codebase already lives (see e.g. id-shape's
// facet/module membership checks).
var validBuildRoles = map[model.BuildRole]bool{
	model.BuildRoleOrientation:  true,
	model.BuildRoleSchema:       true,
	model.BuildRoleBehavior:     true,
	model.BuildRoleAPI:          true,
	model.BuildRoleVerification: true,
	model.BuildRoleOutOfScope:   true,
}

func (BuildRoleRequiredLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	adoptedModule := hasAdoptedBuildRole(claims)

	var findings []Finding
	for _, c := range claims {
		if c.BuildRole == "" {
			if c.Status == model.StatusLocked && adoptedModule[c.Module] {
				findings = append(findings, Finding{
					LintName: "build-role-required-for-locked",
					ClaimID:  c.ID,
					Message:  "build_role is required once a claim is locked",
				})
			}
			continue
		}
		if !validBuildRoles[c.BuildRole] {
			findings = append(findings, Finding{
				LintName: "build-role-required-for-locked",
				ClaimID:  c.ID,
				Message:  "invalid build_role " + string(c.BuildRole),
			})
		}
	}
	return findings
}

// hasAdoptedBuildRole returns, per module, whether at least one claim in
// that module sets a non-empty BuildRole at all. It is the signal
// BuildRoleRequiredLint.Check uses to scope its "locked claim missing
// build_role" finding to modules that have actually started using the
// field — see this file's package doc comment for why that scoping exists.
func hasAdoptedBuildRole(claims []model.Claim) map[string]bool {
	adopted := make(map[string]bool)
	for _, c := range claims {
		if c.BuildRole != "" {
			adopted[c.Module] = true
		}
	}
	return adopted
}
