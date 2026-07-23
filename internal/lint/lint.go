// Package lint defines the Lint interface every one of the 21 lints
// (dangling, ambiguous, id-shape, rest-on-locked, cycle, governed-required,
// mirror-mismatch, mirror-unanchored, mirror-reciprocal, rows-shape,
// supersede, raw-html-scope, roll-up, validated-on-missing, body-edge-hint,
// code-orphan, orphan, layout-shape-mismatch, build-role-required,
// orientation-note-order, orientation-note-shape) implements, one per file
// in this package. This file only defines the contract and the registry;
// individual lint implementations are a later phase and Registry starts
// empty on purpose.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// Severity distinguishes a hard failure from a report-only observation.
// This field did not exist on the scaffolded Finding struct; it was added
// here (internal/lint, where Finding actually lives — not internal/model)
// by the second lint-implementation phase because the "orphan" lint is
// spec'd as a WARNING, not an error, and callers (dossierx lint's exit code,
// "dossierx lock"'s lint gate in internal/lock.Lock) need a way to tell the two
// apart. Any lint that doesn't set Severity explicitly reports as
// SeverityError, preserving the original all-findings-are-failures
// behavior for the lints that came before this field existed.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Finding is one problem reported by a Lint against a specific claim.
type Finding struct {
	LintName string
	ClaimID  string
	Message  string
	Severity Severity
}

// Lint is implemented once per rule under internal/lint/. Check must not
// panic on empty claims and must not mutate claims or cfg.
type Lint interface {
	Name() string
	Check(claims []model.Claim, cfg *config.Config) []Finding
}

// Registry is the set of all lints the CLI runs. It is empty until each
// rule's file registers itself (typically via an init() that appends to
// Registry). "dossierx lint" against zero claims and an empty Registry must
// still exit 0 with zero findings.
var Registry []Lint

// RunAll runs every registered lint against claims and returns the
// concatenation of their findings, in Registry order. Safe to call with a
// nil or empty claims slice and/or an empty Registry.
func RunAll(claims []model.Claim, cfg *config.Config) []Finding {
	var findings []Finding
	for _, l := range Registry {
		findings = append(findings, l.Check(claims, cfg)...)
	}
	return findings
}
