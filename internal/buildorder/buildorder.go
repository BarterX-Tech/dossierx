// Package buildorder computes a module's build (implementation) order: a
// different, additional ordering concept from internal/render's "reading
// order" (orderClaims/newGroup) — this package answers "in what sequence
// should a human or agent actually BUILD this module's claims", not "in
// what sequence should a reader scroll through them".
//
// The sequence is a fixed phase list (Phases below), driven by each
// claim's model.BuildRole: orientation claims are read but never acted on;
// schema claims (data shapes) are built first among the "real work"
// phases; behavior claims (workflow/logic — the bulk of the implementation)
// come next, internally ordered by their rests_on edges; api claims
// (public entry points) are built after the behavior they call into;
// verification claims (test checklists/acceptance criteria) are read last,
// once everything else exists to write tests against. model.BuildRoleOutOfScope
// claims are never placed in the sequence at all — they are deferred/
// future-scope — but are still reported (as Artifact.Excluded) so nothing
// silently vanishes from view.
//
// Propose is the pure, in-memory computation (mirroring catalog.Build):
// it never touches disk. The sibling store.go handles the artifact's
// on-disk lifecycle (propose -> write, status, lock -> hash-snapshot),
// mirroring internal/lock's Store/ContentHash precedent for "a generated
// artifact with a human-confirmed lock step that can go stale".
package buildorder

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// Phases is the fixed, engine-owned build-order phase sequence. It is
// exported so callers (and tests) can enumerate it without duplicating the
// list; model.BuildRoleOutOfScope is deliberately never a member — it is
// excluded from the sequence entirely (see Artifact.Excluded).
var Phases = []model.BuildRole{
	model.BuildRoleOrientation,
	model.BuildRoleSchema,
	model.BuildRoleBehavior,
	model.BuildRoleAPI,
	model.BuildRoleVerification,
}

// phaseIndex maps a phase-sequence BuildRole to its position in Phases, for
// "is target's phase later than mine" comparisons. model.BuildRoleOutOfScope
// and any invalid/empty value are deliberately absent — both are handled as
// special cases by their callers rather than being assigned a fake index.
var phaseIndex = func() map[model.BuildRole]int {
	m := make(map[model.BuildRole]int, len(Phases))
	for i, p := range Phases {
		m[p] = i
	}
	return m
}()

// ClaimEntry is one claim's entry inside a PhaseBlock: its id, the source
// file it was loaded from (model.Claim.SourcePath, so a reviewer can jump
// straight to the file), and its full rests_on edge list. RestsOn is kept
// verbatim (same-module and cross-module targets alike) rather than
// filtered down to same-module edges: cross-module edges are informational
// (they never affect this module's own placement — see Propose's doc
// comment) but are still worth showing a reader, exactly like
// components.edgesHTML shows every rests_on edge regardless of what it
// points at.
type ClaimEntry struct {
	ID      string   `json:"id"`
	File    string   `json:"file"`
	RestsOn []string `json:"rests_on,omitempty"`
}

// PhaseBlock is one phase's ordered claim list. Phase is the string form of
// a model.BuildRole (one of Phases) so the artifact reads standalone as
// JSON without a reader needing this package's Go types.
type PhaseBlock struct {
	Phase  string       `json:"phase"`
	Claims []ClaimEntry `json:"claims"`
}

// Artifact is one module's build-order document: its computed phase
// sequence, the claims excluded from it (build_role: out-of-scope), and the
// lock/staleness bookkeeping store.go's Lock/Status add on top. See this
// package's doc comment and store.go's for the propose/lock lifecycle.
//
// Hashes is only ever populated by a successful Lock: it snapshots
// lock.ContentHash for every claim covered by Phases (see ClaimIDs) as of
// that lock, exactly mirroring internal/lock.Store.Hashes but scoped to
// this one module's artifact instead of the whole project. It is empty
// (omitted from JSON) on a freshly-proposed, not-yet-locked artifact.
type Artifact struct {
	Module   string            `json:"module"`
	Locked   bool              `json:"locked"`
	LockedAt string            `json:"locked_at,omitempty"`
	Stale    bool              `json:"stale"`
	StaleIDs []string          `json:"stale_claim_ids,omitempty"`
	Excluded []string          `json:"excluded"`
	Phases   []PhaseBlock      `json:"phases"`
	Hashes   map[string]string `json:"hashes,omitempty"`
}

// ClaimIDs returns every claim id covered by a's Phases (i.e. everything
// actually placed in the build order — Excluded ids are deliberately not
// included), in phase order. It is the set store.go's Lock/Status snapshot
// and re-check hashes against.
func (a *Artifact) ClaimIDs() []string {
	if a == nil {
		return nil
	}
	var ids []string
	for _, p := range a.Phases {
		for _, c := range p.Claims {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// Propose computes module's build order from claims (every claim in the
// project, not pre-filtered — Propose does the module filtering itself, so
// it can also see other modules' claims for the cross-module-edge
// distinction below). It performs no I/O; store.go's WriteArtifact persists
// the result, mirroring catalog.Build/catalog.WriteJSON's split.
//
// Propose enforces three gates, in order, each returning a descriptive,
// non-nil error rather than silently producing a broken or partial order:
//
//  1. Completeness gate — every claim belonging to module (across every
//     facet) must be model.StatusLocked. This makes "dossierx build-order
//     propose" a once-a-module's-docs-are-fully-locked operation: the
//     resulting error lists every still-non-locked claim id, so a reviewer
//     knows exactly what's left.
//  2. Phase-order validation — if a claim in phase P rests_on another claim
//     IN THE SAME MODULE whose BuildRole is a later phase than P, that is a
//     modeling error (the dependency graph doesn't respect the fixed phase
//     sequence), and is reported by name (both claim ids and both phases)
//     rather than silently placed wrong. A rests_on edge to a claim outside
//     module is informational only: cross-module dependencies are out of
//     scope for this module's own build sequence (they may belong to a
//     module whose own build order hasn't even been proposed yet), so they
//     never block placement or count as a phase-order violation.
//  3. Cycle gate — Propose does not require (or itself invoke) the "cycle"
//     lint to have run first, so it cannot simply trust that the rests_on
//     graph is acyclic. Each phase's bucket is layered via a same-module/
//     same-phase-restricted topological sort (see layeredTopoSort); if that
//     sort cannot place every claim in the bucket, the unplaced claims form
//     a cycle, and Propose fails with their ids rather than writing an
//     artifact that silently omits them.
//
// Once all three gates pass, claims are split into Excluded (build_role:
// out-of-scope) and one bucket per Phases entry, and each bucket's
// layeredTopoSort result becomes that phase's final claim order.
func Propose(claims []model.Claim, cfg *config.Config, module string) (*Artifact, error) {
	if strings.TrimSpace(module) == "" {
		return nil, fmt.Errorf("buildorder: module must not be empty")
	}

	var moduleClaims []model.Claim
	for _, c := range claims {
		if c.Module == module {
			moduleClaims = append(moduleClaims, c)
		}
	}
	if len(moduleClaims) == 0 {
		return nil, fmt.Errorf("buildorder: module %q has no claims", module)
	}

	// 1. Completeness gate.
	var notLocked []string
	for _, c := range moduleClaims {
		if c.Status != model.StatusLocked {
			notLocked = append(notLocked, c.ID)
		}
	}
	if len(notLocked) > 0 {
		sort.Strings(notLocked)
		return nil, fmt.Errorf(
			"buildorder: module %q is not fully locked yet; %d claim(s) still non-locked: %s",
			module, len(notLocked), strings.Join(notLocked, ", "),
		)
	}

	// 2. Phase-order validation + 3. cycle gate + the split/order derivation
	// all live in computePhases, the pure routine store.go's recomputeStale
	// also calls so a locked artifact can never silently describe a different
	// order than a fresh Propose would now compute.
	phaseBlocks, excluded, err := computePhases(claims, cfg, module)
	if err != nil {
		return nil, err
	}

	return &Artifact{
		Module:   module,
		Locked:   false,
		Excluded: excluded,
		Phases:   phaseBlocks,
	}, nil
}

// computePhases is the pure, deterministic phase/order/excluded/File
// derivation extracted from Propose so Propose and store.go's recomputeStale
// share ONE routine and can never diverge on what order a given claim set
// produces — the whole point of the staleness re-derivation (a locked artifact
// must never silently describe a different build order than a fresh propose
// would now compute). It takes the FULL claim set (not pre-filtered to module —
// it filters itself, so it can still see other modules' claims for the
// same-module-vs-cross-module edge distinction below), cfg (for displayPath's
// project-relative ClaimEntry.File), and the target module.
//
// It deliberately performs NONE of Propose's side effects or preconditions —
// the completeness gate (error-on-unlocked), the empty-module check, and
// artifact writing all remain in Propose/store.go. It DOES keep the
// derivation's own structural validation (build_role classification,
// phase-order, cycle), returning a descriptive error rather than emitting a
// broken order, since both callers must fail loudly on a claim set with no
// valid order at all.
func computePhases(claims []model.Claim, cfg *config.Config, module string) ([]PhaseBlock, []string, error) {
	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		byID[c.ID] = c
	}
	var moduleClaims []model.Claim
	for _, c := range claims {
		if c.Module == module {
			moduleClaims = append(moduleClaims, c)
		}
	}

	// Split into excluded (out-of-scope) and per-phase buckets, validating
	// every claim's BuildRole is one of the 6 known values as we go (the
	// build-role-required-for-locked lint is the review-time version of
	// this same check; we re-check defensively since we must never trust an
	// unlinted claim set).
	excluded := make([]string, 0)
	phaseClaims := make(map[model.BuildRole][]model.Claim, len(Phases))
	for _, c := range moduleClaims {
		switch {
		case c.BuildRole == model.BuildRoleOutOfScope:
			excluded = append(excluded, c.ID)
		case c.BuildRole == "":
			return nil, nil, fmt.Errorf("buildorder: claim %q is locked but has no build_role set", c.ID)
		case isKnownPhase(c.BuildRole):
			phaseClaims[c.BuildRole] = append(phaseClaims[c.BuildRole], c)
		default:
			return nil, nil, fmt.Errorf("buildorder: claim %q has invalid build_role %q", c.ID, c.BuildRole)
		}
	}
	sort.Strings(excluded)

	// Phase-order validation (same-module edges only).
	for _, c := range moduleClaims {
		if c.BuildRole == model.BuildRoleOutOfScope || c.BuildRole == "" {
			continue
		}
		cPhase := phaseIndex[c.BuildRole]
		for _, dep := range c.RestsOn {
			target, ok := byID[dep]
			if !ok || target.Module != module {
				continue // dangling (dangling lint's concern) or cross-module (informational only).
			}
			tPhase, ok := phaseIndex[target.BuildRole]
			if !ok {
				continue // target is out-of-scope (or, defensively, unset) — not part of the phase sequence.
			}
			if tPhase > cPhase {
				return nil, nil, fmt.Errorf(
					"buildorder: phase-order violation: claim %q (phase %q) rests_on %q (phase %q), which comes later in the fixed phase sequence %v",
					c.ID, c.BuildRole, dep, target.BuildRole, Phases,
				)
			}
		}
	}

	// Layer each phase's bucket via the restricted topological sort.
	var phaseBlocks []PhaseBlock
	for _, phase := range Phases {
		bucket := phaseClaims[phase]
		if len(bucket) == 0 {
			continue
		}
		ordered, cyclic := layeredTopoSort(stableDisplayOrder(bucket))
		if len(cyclic) > 0 {
			return nil, nil, fmt.Errorf(
				"buildorder: phase %q: rests_on cycle detected among %d claim(s): %s (run \"dossierx lint\" for the full cycle path)",
				phase, len(cyclic), strings.Join(cyclic, ", "),
			)
		}
		entries := make([]ClaimEntry, 0, len(ordered))
		for _, c := range ordered {
			entries = append(entries, ClaimEntry{
				ID:      c.ID,
				File:    displayPath(cfg, c.SourcePath),
				RestsOn: append([]string(nil), c.RestsOn...),
			})
		}
		phaseBlocks = append(phaseBlocks, PhaseBlock{Phase: string(phase), Claims: entries})
	}

	return phaseBlocks, excluded, nil
}

// isKnownPhase reports whether role is one of Phases' 5 in-sequence
// values. It exists only so Propose's switch above can express "a known,
// in-sequence phase" as a single guarded case without repeating the
// phaseIndex lookup's zero-value ambiguity (index 0 is both "orientation"
// and Go's map-miss zero value, so a bare `phaseIndex[c.BuildRole] >= 0`
// alone can't distinguish "orientation" from "not present").
func isKnownPhase(role model.BuildRole) bool {
	_, ok := phaseIndex[role]
	return ok
}

// displayPath renders source (a model.Claim.SourcePath, always absolute —
// see internal/config.LoadConfig's ClaimsDir resolution) relative to cfg's
// own directory for ClaimEntry.File, instead of storing the raw absolute
// filesystem path. The artifact (and the viewer partial that renders it,
// build_order.html) is meant to be shareable project documentation, not a
// dump of the reviewing machine's local directory layout — an absolute
// path bakes in the local username/home-directory structure (e.g. on
// macOS, /Users/<name>/...), which has no business appearing in published
// docs. Falls back to the original absolute path only if it turns out to
// not be inside cfg's directory at all (defensive; every real claim's
// SourcePath is under ClaimsDir, itself under cfg.Dir()).
func displayPath(cfg *config.Config, source string) string {
	if cfg == nil {
		return source
	}
	rel, err := filepath.Rel(cfg.Dir(), source)
	if err != nil || strings.HasPrefix(rel, "..") {
		return source
	}
	return rel
}

// stableDisplayOrder mirrors internal/render.orderClaims' Order-then-
// source-order tiebreak — claims with a positive Order sort ascending by
// it, ahead of every unordered (Order == 0) claim, which keeps its
// incoming relative order as a stable secondary key — without that
// function's per-Section run splitting, which is a viewer-only reading-
// order concept with no bearing on build sequencing. It is what
// layeredTopoSort uses to break ties between claims in the same layer (see
// that function's doc comment), so ties resolve to the same "how a human
// actually laid the source out" order the viewer itself uses, per this
// task's requirement to reuse/replicate that idea rather than invent a new
// one.
//
// Callers are expected to pass claims already in a stable base order (e.g.
// loader.LoadClaims' SourcePath order) so the "incoming relative order"
// fallback above means something; stableDisplayOrder itself does not sort
// by SourcePath directly; Order sorting relies on sort.SliceStable to just
// preserve whatever order claims arrived in for every unordered claim.
func stableDisplayOrder(claims []model.Claim) []model.Claim {
	out := make([]model.Claim, len(claims))
	copy(out, claims)
	sort.SliceStable(out, func(i, j int) bool {
		oi, oj := out[i].Order, out[j].Order
		if oi == oj {
			return false
		}
		if oi == 0 {
			return false
		}
		if oj == 0 {
			return true
		}
		return oi < oj
	})
	return out
}

// layeredTopoSort implements Kahn's algorithm restricted to same-module,
// same-phase rests_on edges among claims (claims is assumed to already be
// one phase's, one module's bucket — Propose is the only caller and
// guarantees this): repeatedly peel off every not-yet-placed claim whose
// same-phase rests_on targets (present in claims) are already placed,
// forming successive layers, until every claim is placed. A rests_on B
// means claim A depends on B remaining true (per FORMAT.md), so B —
// the target — is placed first; a claim with no in-bucket rests_on targets
// at all has nothing to wait on and is ready in layer 0.
//
// Ties within one layer break on claims' own incoming order (expected to
// already be stableDisplayOrder'd by the caller), by construction: each
// layer is built by scanning claims in its given order and collecting every
// currently-ready one, so two claims that become ready in the same round
// keep their relative stableDisplayOrder position.
//
// The whole-project rests_on graph is expected to already be acyclic by the
// "cycle" lint (internal/lint/cycle.go) having been run and passed — any
// subgraph of a DAG is itself a DAG, so a same-module/same-phase
// restriction of it can never contain a cycle in a lint-clean project. But
// Propose has no way to know the lint actually ran (nothing enforces that
// ordering), so layeredTopoSort cannot simply trust the invariant: if a
// layer ever comes up empty while claims remain unplaced, that is a
// same-phase/same-module rests_on cycle among exactly those remaining
// claims, and they are returned (sorted, as the second return value) for
// the caller to turn into a hard error instead of silently vanishing from
// the artifact.
func layeredTopoSort(claims []model.Claim) (ordered []model.Claim, cyclic []string) {
	idSet := make(map[string]bool, len(claims))
	for _, c := range claims {
		idSet[c.ID] = true
	}

	// deps[id] = this claim's rests_on targets that are also in this
	// bucket (same phase, same module); dependents[id] = the reverse edge
	// list, used to decrement remaining[] as each target is placed.
	deps := make(map[string][]string, len(claims))
	dependents := make(map[string][]string, len(claims))
	remaining := make(map[string]int, len(claims))
	for _, c := range claims {
		for _, d := range c.RestsOn {
			if idSet[d] {
				deps[c.ID] = append(deps[c.ID], d)
				dependents[d] = append(dependents[d], c.ID)
			}
		}
		remaining[c.ID] = len(deps[c.ID])
	}

	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		byID[c.ID] = c
	}

	placed := make(map[string]bool, len(claims))
	out := make([]model.Claim, 0, len(claims))
	for len(placed) < len(claims) {
		var layer []string
		for _, c := range claims {
			if !placed[c.ID] && remaining[c.ID] == 0 {
				layer = append(layer, c.ID)
			}
		}
		if len(layer) == 0 {
			// Cycle guard — see doc comment above. Every claim not yet
			// placed at this point is part of (or blocked only by) a
			// same-phase/same-module rests_on cycle; report them rather
			// than looping forever or dropping them from the result.
			for _, c := range claims {
				if !placed[c.ID] {
					cyclic = append(cyclic, c.ID)
				}
			}
			sort.Strings(cyclic)
			return out, cyclic
		}
		for _, id := range layer {
			placed[id] = true
			out = append(out, byID[id])
		}
		for _, id := range layer {
			for _, dep := range dependents[id] {
				remaining[dep]--
			}
		}
	}
	return out, nil
}
