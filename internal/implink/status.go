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

// codeProducingRoles is the subset of model.BuildRole phases expected to
// produce real code or tests once a module's claims lock: the same 4
// "real work" phases internal/buildorder.Phases places after orientation,
// minus orientation itself (context/process claims are never expected to
// have a linked file the way a schema/behavior/api/verification claim is)
// and minus out-of-scope (deferred/future-scope, excluded from the build
// order sequence for the same reason). Status uses this set to decide
// which locked-but-unlinked claims are actually worth reporting as
// "unlinked" — an orientation or out-of-scope claim missing a linked file
// is the normal, expected case, not a gap.
var codeProducingRoles = map[model.BuildRole]bool{
	model.BuildRoleSchema:       true,
	model.BuildRoleBehavior:     true,
	model.BuildRoleAPI:          true,
	model.BuildRoleVerification: true,
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

// StatusReport is Status's full result for one module: how many claims
// have at least one linked file, which specific linked files have drifted,
// and how many of the module's locked, code-producing-phase claims have no
// linked file at all. UnlinkedCount is deliberately always present (never
// omitted or hidden behind a "no drift, nothing to see" summary) — a
// project adopting this feature needs to see gaps as loudly as it sees
// drift.
type StatusReport struct {
	Module        string
	LinkedClaims  int
	Drifted       []DriftEntry
	UnlinkedCount int
	UnlinkedIDs   []string
}

// Summary returns a one-line human-readable roll-up of r, in the exact
// wording both "dossierx check"'s non-blocking impl-links step and "dossierx
// implink status" print, so the two call sites can never drift apart on
// phrasing.
func (r *StatusReport) Summary() string {
	return fmt.Sprintf(
		"impl-links: %d linked, %d drifted, %d unlinked-in-schema/behavior/api/verification-phases",
		r.LinkedClaims, len(r.Drifted), r.UnlinkedCount,
	)
}

// Status loads module's implementation-link artifact and reports its
// current state against claims: every linked file's hash is re-checked
// against its stored baseline (a mismatch, or the file having vanished
// entirely, is reported as drift), and every one of module's locked claims
// whose build_role is a code-producing phase (see codeProducingRoles) but
// which has no Link entry at all is counted as unlinked. It never writes
// path back out — recomputing drift is a pure read, exactly like
// buildorder.Status never mutates the artifact it loads.
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

	report := &StatusReport{Module: module, LinkedClaims: len(artifact.Links)}

	linked := make(map[string]bool, len(artifact.Links))
	for _, link := range artifact.Links {
		linked[link.ClaimID] = true
		for _, f := range link.Files {
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
		if c.Module != module || c.Status != model.StatusLocked {
			continue
		}
		if !codeProducingRoles[c.BuildRole] {
			continue
		}
		if !linked[c.ID] {
			report.UnlinkedIDs = append(report.UnlinkedIDs, c.ID)
		}
	}
	sort.Strings(report.UnlinkedIDs)
	report.UnlinkedCount = len(report.UnlinkedIDs)

	return report, nil
}

// ViewFile is one linked file annotated with its current drift status —
// the shape internal/render's viewer wiring actually wants for the shared
// edges footer's "implemented in" lines (see components.edgesHTML), rather
// than the raw FileLink plus a separate drift lookup the caller would
// otherwise have to build for itself by duplicating Status's hash-recompute
// logic.
type ViewFile struct {
	File    string
	Symbol  string
	Drifted bool
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
			views = append(views, ViewFile{File: f.File, Symbol: f.Symbol, Drifted: drifted})
		}
		out[link.ClaimID] = views
	}
	return out, nil
}
