// Package catalog builds the intermediate representation ("Catalog") that
// internal/render consumes to produce the viewer, and that can be
// serialized to .catalog.json for external tooling. Build groups claims by
// facet/module, infers a layout where one is missing, and (in Document)
// projects each claim down to the id/facet/module/status/layout/edges shape
// that ships on disk.
//
// Go's map iteration order is randomized per-run, so nothing in this
// package ever writes JSON output by ranging over a map directly: every
// slice or map that reaches Document (and therefore WriteJSON) is built via
// an explicit sort first. That's what makes two builds from the same input
// byte-identical.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/readiness"
)

// Catalog is the built, render-ready view over a set of claims.
type Catalog struct {
	Claims    []model.Claim
	Readiness map[string]readiness.Assessment

	// ByFacet and ByModule group claim IDs for convenient lookup by later
	// render/lint stages. Populated by Build. Each slice of IDs is sorted so
	// callers never need to re-sort before using or serializing them.
	ByFacet  map[string][]string
	ByModule map[string][]string
}

// SetReadiness attaches a current read-only approval projection for exports
// and offline rendering. Build remains a pure authored-claim projection.
func (cat *Catalog) SetReadiness(assessments map[string]readiness.Assessment) {
	if cat != nil {
		cat.Readiness = assessments
	}
}

// Build validates nothing beyond what's needed to construct a Catalog
// (lint is responsible for correctness checks) and never panics on empty
// input: Build(nil, cfg) returns an empty, valid Catalog.
func Build(claims []model.Claim, cfg *config.Config) (*Catalog, error) {
	cat := &Catalog{
		Claims:   make([]model.Claim, 0, len(claims)),
		ByFacet:  map[string][]string{},
		ByModule: map[string][]string{},
	}

	for _, c := range claims {
		if c.Layout == "" {
			c.Layout = inferLayout(c)
		}
		cat.Claims = append(cat.Claims, c)
		if c.Facet != "" {
			cat.ByFacet[c.Facet] = append(cat.ByFacet[c.Facet], c.ID)
		}
		if c.Module != "" {
			cat.ByModule[c.Module] = append(cat.ByModule[c.Module], c.ID)
		}
	}

	for _, ids := range cat.ByFacet {
		sort.Strings(ids)
	}
	for _, ids := range cat.ByModule {
		sort.Strings(ids)
	}

	return cat, nil
}

// inferLayout implements the shape-based inference described in
// FORMAT.md: rows present -> table; a non-empty Steps -> steps;
// otherwise -> card.
func inferLayout(c model.Claim) model.Layout {
	switch {
	case len(c.Rows) > 0:
		return model.LayoutTable
	case len(c.Steps) > 0:
		return model.LayoutSteps
	default:
		return model.LayoutCard
	}
}

// GovernedEdge is the serialized form of a claim's governed_by edge.
type GovernedEdge struct {
	Type   string `json:"type"`
	Reason string `json:"reason,omitempty"`
}

// Edges is the serialized edge graph for one claim entry.
type Edges struct {
	Mirrors    []string      `json:"mirrors,omitempty"`
	RestsOn    []string      `json:"rests_on,omitempty"`
	GovernedBy *GovernedEdge `json:"governed_by,omitempty"`
}

// Entry is the .catalog.json projection of a single claim: id/facet/module/
// status/layout plus its outgoing edges. It deliberately omits body/rows/
// steps — those are render concerns, not catalog concerns.
type Entry struct {
	ID     string       `json:"id"`
	Facet  string       `json:"facet"`
	Module string       `json:"module"`
	Status model.Status `json:"status"`
	Layout model.Layout `json:"layout"`

	// Kind is c.EffectiveKind() — the resolved kind, not the raw
	// (possibly-unset) model.Claim.Kind field — so a .catalog.json
	// consumer never has to re-derive the "overview facet implies
	// orientation-note" rule itself.
	Kind model.Kind `json:"kind"`

	Edges Edges `json:"edges"`

	// Tracks is the claim's cross-cutting membership, carried here because
	// it is STRUCTURE — which named concerns this claim participates in, and
	// in which role — the same category as Edges, and the thing a consumer
	// asking "what makes up this feature" needs. It is deliberately not
	// modelled inside Edges: those are claim-to-claim semantic dependencies
	// with cycle lints attached, and membership is neither.
	//
	// `omitempty` is load-bearing for the same reason it is on the claim
	// field: a project that declares no tracks writes a .catalog.json
	// byte-identical to the one it wrote before tracks existed.
	//
	// Sources are deliberately NOT projected here. .catalog.json omits
	// body/rows/steps because they are render concerns rather than catalog
	// structure, and a claim's evidence sits on that same side of the line —
	// it is read by a human on the claim, not resolved by a consumer of the
	// index.
	Tracks    []TrackMembership     `json:"tracks,omitempty"`
	Readiness *readiness.Assessment `json:"readiness,omitempty"`
}

// TrackMembership is the serialized form of one claim's membership in one
// track. Role is always written out explicitly — resolved through
// model.TrackRef.EffectiveRole rather than copied raw — so a consumer never
// has to know that an absent role means "cites", exactly as Entry.Kind
// resolves the overview-facet rule rather than exporting the raw field.
type TrackMembership struct {
	ID   string          `json:"id"`
	Role model.TrackRole `json:"role"`
}

// Document is the full on-disk .catalog.json shape.
type Document struct {
	Claims   []Entry             `json:"claims"`
	ByFacet  map[string][]string `json:"by_facet"`
	ByModule map[string][]string `json:"by_module"`
}

// entryFor projects one claim into its Entry form.
func entryFor(c model.Claim) Entry {
	e := Entry{
		ID:     c.ID,
		Facet:  c.Facet,
		Module: c.Module,
		Status: c.Status,
		Layout: c.Layout,
		Kind:   c.EffectiveKind(),
	}

	if len(c.Mirrors) > 0 {
		e.Edges.Mirrors = append([]string(nil), c.Mirrors...)
	}
	if len(c.RestsOn) > 0 {
		e.Edges.RestsOn = append([]string(nil), c.RestsOn...)
	}
	if c.Governed.Type != "" {
		e.Edges.GovernedBy = &GovernedEdge{
			Type:   c.Governed.Type,
			Reason: c.Governed.Reason,
		}
	}

	for _, t := range c.Tracks {
		e.Tracks = append(e.Tracks, TrackMembership{ID: t.ID, Role: t.EffectiveRole()})
	}

	return e
}

// Document builds the deterministic .catalog.json projection of cat: one
// Entry per claim, sorted by id (never by Go map order, which is not
// stable), plus copies of ByFacet/ByModule with each id slice sorted.
//
// Document never panics on an empty catalog: an empty (or nil) Catalog
// produces a Document with an empty (non-nil) Claims slice and empty
// (non-nil) ByFacet/ByModule maps.
func (cat *Catalog) Document() *Document {
	doc := &Document{
		Claims:   make([]Entry, 0),
		ByFacet:  map[string][]string{},
		ByModule: map[string][]string{},
	}
	if cat == nil {
		return doc
	}

	for _, c := range cat.Claims {
		e := entryFor(c)
		if assessment, ok := cat.Readiness[c.ID]; ok {
			copy := assessment
			e.Readiness = &copy
		}
		doc.Claims = append(doc.Claims, e)
	}
	sort.Slice(doc.Claims, func(i, j int) bool { return doc.Claims[i].ID < doc.Claims[j].ID })

	for facet, ids := range cat.ByFacet {
		sorted := append([]string(nil), ids...)
		sort.Strings(sorted)
		doc.ByFacet[facet] = sorted
	}
	for module, ids := range cat.ByModule {
		sorted := append([]string(nil), ids...)
		sort.Strings(sorted)
		doc.ByModule[module] = sorted
	}

	return doc
}

// MarshalJSON deterministically serializes cat by delegating to Document,
// so json.Marshal(cat) and cat.WriteJSON both round-trip through the same
// sorted projection.
func (cat *Catalog) MarshalJSON() ([]byte, error) {
	return json.Marshal(cat.Document())
}

// WriteJSON serializes cat's Document to path as indented JSON, creating
// path's parent directory if needed. Output is deterministic: building the
// same claims twice and writing both produces byte-identical files.
func WriteJSON(cat *Catalog, path string) error {
	if cat == nil {
		cat = &Catalog{}
	}

	data, err := json.MarshalIndent(cat.Document(), "", "  ")
	if err != nil {
		return fmt.Errorf("catalog: marshal: %w", err)
	}
	data = append(data, '\n')

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("catalog: create output dir %q: %w", dir, err)
		}
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("catalog: write %q: %w", path, err)
	}

	return nil
}
