// status.go implements the read-only reporting half of this package's
// contract: Status recomputes drift for every linked file against its
// stored baseline hash and separately counts a module's locked,
// code-producing-phase claims that have no linked file at all — mirroring
// internal/buildorder's Status (a read-only recompute-on-load over an
// artifact only ever written elsewhere, by Lock there and by Set here).
package implink

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// Expects reports whether c is a claim the code-link gate holds to account:
// a locked MODULE claim that has not declared itself code-free. A draft claim
// is never expected to be linked (Scan refuses a tag on it); a project claim
// (project.<slug>) is project-wide law with no module to own its code; and a
// claim carrying `embodiment: {mode: none, reason: ...}` has recorded, under
// the human's lock, that it has no software embodiment. Every other locked
// claim is expected to point at the code or test that makes it true — the
// one predicate Status's unlinked count, check's code-link gate and the
// viewer's "not linked to code" row all key off.
func Expects(c model.Claim) bool {
	if c.Status != model.StatusLocked || c.IsProjectClaim() {
		return false
	}
	return c.Embodiment == nil || c.Embodiment.Mode != model.EmbodimentModeNone
}

// DriftEntry is one linked file whose current on-disk content no longer
// matches the hash snapshotted at its last Set call. Reason is a one-line,
// human-readable explanation only — never a real content diff, since this
// package is language-agnostic and only ever hashes whole files; it cannot
// know *what* changed inside one, only *that* something did (or that the
// file has disappeared entirely).
type DriftEntry struct {
	ClaimID string
	File    string
	Reason  string
}

// PartialEntry is one locked claim that carries `steps:` and has at least
// one linked file, but whose dossierx-step tags do not cover every step.
// Covered counts the DISTINCT 1-based step indexes that some tag attested;
// Total is len(claim.Steps); Missing lists the untagged indexes in order.
// A whole-claim link (dossierx-claim, or `claim link`; FileLink.Step == 0)
// never counts toward Covered: it grounds the file for drift, and it is
// exactly the "one tag clears the whole claim" shape the gate refuses.
type PartialEntry struct {
	ClaimID string `json:"claim_id"`
	Covered int    `json:"covered"`
	Total   int    `json:"total"`
	Missing []int  `json:"missing"`
}

// StatusReport is Status's full result for one module: how many claims
// have at least one linked file, which specific linked files have drifted,
// which stepped claims are linked but not on every step, and how many of
// the module's locked, code-producing-phase claims have no linked file at
// all. UnlinkedCount and PartialCount are deliberately always present
// (never omitted or hidden behind a "no drift, nothing to see" summary) — a
// project adopting this feature needs to see gaps as loudly as it sees
// drift, and check's code-link gate refuses on either.
type StatusReport struct {
	Module        string
	LinkedClaims  int
	Drifted       []DriftEntry
	Partial       []PartialEntry
	PartialCount  int
	UnlinkedCount int
	UnlinkedIDs   []string
}

// Incomplete is the number of claims the code-link gate refuses on: every
// unlinked claim plus every partially-linked one.
func (r *StatusReport) Incomplete() int {
	return r.UnlinkedCount + r.PartialCount
}

// Summary returns a one-line human-readable roll-up of r, in the exact
// wording both "dossierx check"'s impl-links step and "dossierx claim show"
// print, so the two call sites can never drift apart on phrasing.
func (r *StatusReport) Summary() string {
	return fmt.Sprintf(
		"impl-links: %d linked, %d drifted, %d partial, %d unlinked-in-schema/behavior/api/verification-phases",
		r.LinkedClaims, len(r.Drifted), r.PartialCount, r.UnlinkedCount,
	)
}

// StepCoverage folds the step indexes attested by a claim's links against
// its `steps:` count: covered is the number of DISTINCT indexes within
// 1..total that appear in steps, and missing lists the rest in ascending
// order. Indexes outside 1..total and the zero (whole-claim) index are
// ignored — Scan already refused an out-of-range tag, and a whole-claim
// link is not a step attestation. A claim with no steps is trivially
// covered (0 of 0, nothing missing).
func StepCoverage(total int, steps []int) (covered int, missing []int) {
	if total <= 0 {
		return 0, nil
	}
	seen := make([]bool, total+1)
	for _, n := range steps {
		if n >= 1 && n <= total && !seen[n] {
			seen[n] = true
			covered++
		}
	}
	for n := 1; n <= total; n++ {
		if !seen[n] {
			missing = append(missing, n)
		}
	}
	return covered, missing
}

// Status loads module's implementation-link artifact and reports its
// current state against claims: every linked file's hash is re-checked
// against its stored baseline (a mismatch, or the file having vanished
// entirely, is reported as drift), and every one of module's claims that
// Expects holds to account but which has no Link entry at all is counted as
// unlinked. It never writes path back out — recomputing drift is a pure read.
//
// A missing artifact (module has never called Set) returns an error
// wrapping ErrNoArtifact; callers that want to treat that as "nothing to
// report" (e.g. "dossierx check"'s silent-when-unused wiring) should check for
// it via errors.Is.
func Status(claims []model.Claim, cfg *config.Config, module string) (*StatusReport, error) {
	if cfg == nil {
		return nil, fmt.Errorf("implink: cfg must not be nil")
	}
	artifact, err := LoadArtifact(ArtifactPath(cfg, module))
	if err != nil {
		return nil, err
	}
	return status(claims, cfg, module, artifact), nil
}

// Coverage is Status for the code-link gate: a module that has never
// linked anything is not "nothing to report", it is a module in which
// EVERY locked, code-producing claim is unlinked. Where Status wraps
// ErrNoArtifact for a missing artifact — the right answer for a reporter
// that must stay silent on projects that never opted in — Coverage
// evaluates the empty artifact, because the caller has already decided,
// from `source_dirs`, that this project is held to account. Any other load
// error is still returned.
func Coverage(claims []model.Claim, cfg *config.Config, module string) (*StatusReport, error) {
	if cfg == nil {
		return nil, fmt.Errorf("implink: cfg must not be nil")
	}
	artifact, err := LoadArtifact(ArtifactPath(cfg, module))
	if err != nil {
		if !errors.Is(err, ErrNoArtifact) {
			return nil, err
		}
		artifact = &Artifact{Module: module}
	}
	return status(claims, cfg, module, artifact), nil
}

func status(claims []model.Claim, cfg *config.Config, module string, artifact *Artifact) *StatusReport {
	report := &StatusReport{Module: module, LinkedClaims: len(artifact.Links)}

	linked := make(map[string]bool, len(artifact.Links))
	// stepsByClaim collects every step index a link attested, so a claim
	// with `steps:` is judged on step coverage, not on "has any link".
	stepsByClaim := make(map[string][]int, len(artifact.Links))
	for _, link := range artifact.Links {
		linked[link.ClaimID] = true
		for _, f := range link.Files {
			if f.Step > 0 {
				stepsByClaim[link.ClaimID] = append(stepsByClaim[link.ClaimID], f.Step)
			}
			current, statErr := hashFile(filepath.Join(cfg.Dir(), f.File))
			switch {
			case statErr == nil && current == f.FileHash:
				// unchanged — no drift.
			case statErr != nil && errors.Is(statErr, fs.ErrNotExist):
				report.Drifted = append(report.Drifted, DriftEntry{
					ClaimID: link.ClaimID,
					File:    f.File,
					Reason:  fmt.Sprintf("file is missing (was linked at %s)", link.LinkedAt),
				})
			case statErr != nil:
				report.Drifted = append(report.Drifted, DriftEntry{
					ClaimID: link.ClaimID,
					File:    f.File,
					Reason:  fmt.Sprintf("could not re-check file (linked at %s): %s", link.LinkedAt, statErr),
				})
			default:
				report.Drifted = append(report.Drifted, DriftEntry{
					ClaimID: link.ClaimID,
					File:    f.File,
					Reason:  fmt.Sprintf("file changed since linked at %s", link.LinkedAt),
				})
			}
		}
	}
	sort.Slice(report.Drifted, func(i, j int) bool {
		if report.Drifted[i].ClaimID != report.Drifted[j].ClaimID {
			return report.Drifted[i].ClaimID < report.Drifted[j].ClaimID
		}
		return report.Drifted[i].File < report.Drifted[j].File
	})

	for _, c := range claims {
		if c.Module != module || !Expects(c) {
			continue
		}
		if !linked[c.ID] {
			report.UnlinkedIDs = append(report.UnlinkedIDs, c.ID)
			continue
		}
		if total := len(c.Steps); total > 0 {
			covered, missing := StepCoverage(total, stepsByClaim[c.ID])
			if covered < total {
				report.Partial = append(report.Partial, PartialEntry{ClaimID: c.ID, Covered: covered, Total: total, Missing: missing})
			}
		}
	}
	sort.Strings(report.UnlinkedIDs)
	sort.Slice(report.Partial, func(i, j int) bool { return report.Partial[i].ClaimID < report.Partial[j].ClaimID })
	report.UnlinkedCount = len(report.UnlinkedIDs)
	report.PartialCount = len(report.Partial)

	return report
}

// ViewFile is one linked file annotated with its current drift status —
// the shape internal/render's viewer wiring actually wants for the shared
// edges footer's "implemented in" lines (see components.edgesHTML), rather
// than the raw FileLink plus a separate drift lookup the caller would
// otherwise have to build for itself by duplicating Status's hash-recompute
// logic.
type ViewFile struct {
	File     string
	Symbol   string
	Drifted  bool
	Step     int
	StepHash string
}

// ViewsByClaim returns, for every claim id in module's implementation-link
// artifact that has at least one linked file, that claim's files annotated
// with current drift status. It returns an error wrapping ErrNoArtifact
// when module has no artifact at all (never called Set) — callers (namely
// internal/render's attach step) are expected to treat that as "render
// nothing extra for this module", the same graceful-degradation contract
// internal/render's Build order tab follows for a module whose build-order
// artifact does not load.
func ViewsByClaim(cfg *config.Config, module string) (map[string][]ViewFile, error) {
	if cfg == nil {
		return nil, fmt.Errorf("implink: cfg must not be nil")
	}
	artifact, err := LoadArtifact(ArtifactPath(cfg, module))
	if err != nil {
		return nil, err
	}

	out := make(map[string][]ViewFile, len(artifact.Links))
	for _, link := range artifact.Links {
		views := make([]ViewFile, 0, len(link.Files))
		for _, f := range link.Files {
			current, statErr := hashFile(filepath.Join(cfg.Dir(), f.File))
			drifted := statErr != nil || current != f.FileHash
			views = append(views, ViewFile{File: f.File, Symbol: f.Symbol, Drifted: drifted, Step: f.Step, StepHash: f.StepHash})
		}
		out[link.ClaimID] = views
	}
	return out, nil
}
