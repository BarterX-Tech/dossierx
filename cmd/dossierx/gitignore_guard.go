// gitignore_guard.go is the approval verbs' half of the store-gitignored guard
// (internal/check.Gitignored): claim lock (single and batch), claim flag, claim
// reaudit --confirm and build-order lock are about to write an approval whose
// ONLY way to reach a collaborator is the repository, so a store git would
// ignore — or a work tree git cannot answer for — is a refusal, not a skip.
// The read-only check modes take the other branch (they report
// data.gitignore_check and exit 0; see runCheckStaged's doc comment), and
// `check` itself carries the same state as a store-gitignored finding.
package main

import (
	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// refuseIfStoresGitignored runs the guard for an approval-recording verb. It
// returns the ignored-but-tracked warnings (which the verb prints on success)
// and, when any store the verb writes would be ignored or git could not answer
// inside a work tree, the store_gitignored refusal — whose message is the
// first finding's own text (or the git-unavailable text), so the refusal and
// `check`'s finding say the same thing.
func refuseIfStoresGitignored(cfg *config.Config, verb string) ([]string, error) {
	findings, warnings, _, err := check.Gitignored(cfg)
	if err != nil {
		return nil, cliout.Errorf(cliout.CodeStoreGitignored, "%s: %w", verb, err)
	}
	if len(findings) > 0 {
		return nil, cliout.Errorf(cliout.CodeStoreGitignored, "%s: %s", verb, findings[0].Message).
			WithDetails(map[string]any{"findings": findings})
	}
	return warnings, nil
}

// storesArePrecondition is the guard's dry-run twin: the `stores_are_tracked`
// precondition fails in exactly the state the real run refuses with
// store_gitignored, so a preview never promises a write the run then refuses.
func storesArePrecondition(dr *cliout.DryRun, cfg *config.Config) {
	findings, warnings, reason, err := check.Gitignored(cfg)
	switch {
	case err != nil:
		dr.Require("stores_are_tracked", false, err.Error())
	case len(findings) > 0:
		dr.Require("stores_are_tracked", false, findings[0].Message)
	case len(warnings) > 0:
		dr.Require("stores_are_tracked", true, warnings[0])
	case reason != "":
		dr.Require("stores_are_tracked", true, "gitignore check did not apply: "+reason)
	default:
		dr.Require("stores_are_tracked", true, "no store the engine writes under the build directory is ignored by .gitignore")
	}
}
