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
	"sort"
	"unicode/utf8"

	"github.com/BarterX-Tech/dossierx/internal/atomicfile"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/readiness"
)

// Catalog is the built, render-ready view over a set of claims.
type Catalog struct {
	Claims      []model.Claim
	Readiness   map[string]readiness.Assessment
	Conformance map[string]conformance.Result

	// ByFacet and ByModule group claim IDs for convenient lookup by later
	// render/lint stages. Populated by Build. Each slice of IDs is sorted so
	// callers never need to re-sort before using or serializing them.
	ByFacet  map[string][]string
	ByModule map[string][]string
}

// SetConformance attaches the read-only structured implementation projection.
// A nil report leaves the catalog byte-compatible for projects that do not use
// the feature.
func (cat *Catalog) SetConformance(report *conformance.Report) {
	if cat == nil || report == nil {
		return
	}
	cat.Conformance = make(map[string]conformance.Result, len(report.Results))
	for _, result := range report.Results {
		cat.Conformance[result.ClaimID] = result
	}
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
	Tracks      []TrackMembership     `json:"tracks,omitempty"`
	Readiness   *readiness.Assessment `json:"readiness,omitempty"`
	Conformance *conformance.Result   `json:"conformance,omitempty"`
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
			assessmentCopy := assessment
			e.Readiness = &assessmentCopy
		}
		if result, ok := cat.Conformance[c.ID]; ok {
			resultCopy := result
			e.Conformance = &resultCopy
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

// EncodeJSON serializes cat without touching its destination.
func EncodeJSON(cat *Catalog) ([]byte, error) {
	return encodeJSON(cat, 0)
}

// EncodeJSONBounded rejects early only when unavoidable encoded values and
// mandatory JSON structure prove overflow, then measures the actual
// deterministic document.
func EncodeJSONBounded(cat *Catalog, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("catalog: max bytes must be positive")
	}
	if err := preflightCatalogCapacity(cat, uint64(maxBytes)); err != nil {
		return nil, err
	}
	return encodeJSON(cat, maxBytes)
}

// preflightCatalogCapacity walks without constructing a Document or JSON
// buffer. Only a lower bound is refusal authority: padding in the separate
// upper-bound diagnostic can never reject valid output.
func preflightCatalogCapacity(cat *Catalog, outputLimit uint64) error {
	lower := catalogProjectionStringLowerBound(cat, outputLimit)
	if lower.exceeded {
		return fmt.Errorf("%w: catalog output is at least %d bytes: mandatory JSON content exceeds %d bytes, so actual output would be larger; output was not encoded", conformance.ErrCapacityExceeded, outputLimit+1, outputLimit)
	}
	return nil
}

// catalogProjectionStringLowerBound counts exact encoded string values plus
// only field names, delimiters, and indentation that MarshalIndent must emit.
// It deliberately omits optional structure and separators where proving their
// presence would complicate the walk, so crossing the limit remains proof that
// the real document is larger without constructing it.
func catalogProjectionStringLowerBound(cat *Catalog, limit uint64) catalogBudget {
	b := catalogBudget{limit: limit}
	b.add(minimalCatalogJSONBytes)
	add := func(value string) {
		if value != "" {
			b.addString(value)
		}
	}
	addStrings := func(values []string) {
		for _, value := range values {
			add(value)
			if b.exceeded {
				return
			}
		}
	}
	addCondition := func(condition readiness.DependencyCondition) {
		b.add(dependencyConditionStructureBytes)
		add(string(condition.Kind))
		add(condition.DependencyID)
		add(condition.Detail)
		addStrings(condition.Path)
	}
	addCause := func(cause readiness.Cause) {
		b.add(readinessCauseStructureBytes)
		add(string(cause.Kind))
		add(string(cause.SourceKind))
		add(cause.DependencyID)
		add(cause.Detail)
		addStrings(cause.Path)
	}
	if cat == nil {
		return b
	}
	for _, claim := range cat.Claims {
		b.add(catalogEntryStructureBytes)
		for _, value := range []string{claim.ID, claim.Facet, claim.Module, string(claim.Status), string(claim.Layout), string(claim.EffectiveKind())} {
			add(value)
		}
		if claim.Governed.Type != "" {
			add(claim.Governed.Type)
			add(claim.Governed.Reason)
		}
		addStrings(claim.Mirrors)
		addStrings(claim.RestsOn)
		for _, track := range claim.Tracks {
			b.add(trackStructureBytes)
			add(track.ID)
			add(string(track.EffectiveRole()))
		}
		if assessment, ok := cat.Readiness[claim.ID]; ok {
			b.add(readinessAssessmentStructureBytes)
			add(assessment.ClaimID)
			add(assessment.LocalApprovalIssue)
			addStrings(assessment.LocalReasons)
			for _, condition := range assessment.DependencyConditions {
				addCondition(condition)
			}
			for _, condition := range assessment.Conditions {
				addCondition(condition)
			}
			for _, cause := range assessment.ReviewCauses {
				addCause(cause)
			}
			for _, cause := range assessment.Causes {
				addCause(cause)
			}
		}
		if result, ok := cat.Conformance[claim.ID]; ok {
			b.add(conformanceResultStructureBytes)
			if len(result.Checks) > 0 {
				b.add(conformanceChecksFieldStructureBytes)
			}
			if result.Reason != "" {
				b.add(optionalStringFieldStructureBytes)
			}
			for _, value := range []string{result.ClaimID, string(result.Mode), result.Reason} {
				add(value)
			}
			for _, check := range result.Checks {
				b.add(conformanceCheckStructureBytes)
				if check.Expected != nil {
					b.add(optionalValueFieldStructureBytes)
				}
				if check.Observed != nil {
					b.add(optionalValueFieldStructureBytes)
				}
				if len(check.Missing) > 0 {
					b.add(optionalArrayFieldStructureBytes)
				}
				if len(check.Extra) > 0 {
					b.add(optionalArrayFieldStructureBytes)
				}
				if check.Reason != "" {
					b.add(optionalStringFieldStructureBytes)
				}
				if check.ObservationError != nil {
					b.add(observationErrorStructureBytes)
				}
				for _, value := range []string{check.ID, check.Adapter, check.Target, string(check.Shape), string(check.State), check.Reason} {
					add(value)
				}
				switch expected := check.Expected.(type) {
				case string:
					add(expected)
				case []string:
					b.add(2)
					addStrings(expected)
				}
				switch observed := check.Observed.(type) {
				case string:
					add(observed)
				case []string:
					b.add(2)
					addStrings(observed)
				}
				addStrings(check.Missing)
				addStrings(check.Extra)
				if check.ObservationError != nil {
					add(check.ObservationError.Code)
					add(check.ObservationError.Message)
				}
			}
		}
		if b.exceeded {
			return b
		}
	}
	for key, ids := range cat.ByFacet {
		b.add(mapArrayEntryStructureBytes)
		add(key)
		addStrings(ids)
		if b.exceeded {
			return b
		}
	}
	for key, ids := range cat.ByModule {
		b.add(mapArrayEntryStructureBytes)
		add(key)
		addStrings(ids)
		if b.exceeded {
			return b
		}
	}
	return b
}

// The empty document is exactly what MarshalIndent emits before any records
// are inserted, including its trailing newline.
const minimalCatalogJSONBytes = uint64(len(`{
  "claims": [],
  "by_facet": {},
  "by_module": {}
}
`))

const (
	// Catalog entries are array elements at indentation depth two. Values
	// counted separately are intentionally blank in the skeleton.
	catalogEntryStructureBytes  = uint64(len(`{"id":,"facet":,"module":,"status":,"layout":,"kind":,"edges":{}}`) + 5 + 7*(1+6) + 7 + (1 + 4))
	trackStructureBytes         = uint64(len(`{"id":,"role":}`))
	mapArrayEntryStructureBytes = uint64(len(`:[]`))

	readinessAssessmentStructureBytes = uint64(len(`{"claim_id":,"policy_version":0,"local_approved":true,"locally_approved":true,"dependency_ready":true,"ready":true,"review_pending":true}`))
	dependencyConditionStructureBytes = uint64(len(`{"kind":,"path":[]}`))
	readinessCauseStructureBytes      = uint64(len(`{"kind":,"path":[],"direct":true,"inherited":true}`))

	conformanceResultStructureBytes      = uint64(len(`{"claim_id":,"mode":,"implementation_ready":true}`))
	conformanceChecksFieldStructureBytes = uint64(len(`,"checks":[]`))
	conformanceCheckStructureBytes       = uint64(len(`{"id":,"adapter":,"target":,"shape":,"state":,"implementation_ready":true}`))
	optionalStringFieldStructureBytes    = uint64(len(`,"reason":`))
	optionalValueFieldStructureBytes     = uint64(len(`,"expected":`))
	// "extra" is the shortest of the two array field names; using it for both
	// missing and extra keeps the shared charge a strict lower bound.
	optionalArrayFieldStructureBytes = uint64(len(`,"extra":[]`))
	observationErrorStructureBytes   = uint64(len(`,"observation_error":{"code":,"message":}`))
)

func catalogProjectionUpperBound(cat *Catalog, limit uint64) catalogBudget {
	b := catalogBudget{limit: limit}
	b.add(8192)
	if cat == nil {
		return b
	}
	for _, claim := range cat.Claims {
		b.add(2048)
		for _, value := range []string{claim.ID, claim.Facet, claim.Module, string(claim.Status), string(claim.Layout), string(claim.EffectiveKind()), claim.Governed.Type, claim.Governed.Reason} {
			b.addString(value)
		}
		for _, value := range claim.Mirrors {
			b.addString(value)
			b.add(64)
		}
		for _, value := range claim.RestsOn {
			b.addString(value)
			b.add(64)
		}
		for _, track := range claim.Tracks {
			b.addString(track.ID)
			b.addString(string(track.EffectiveRole()))
			b.add(128)
		}
		if assessment, ok := cat.Readiness[claim.ID]; ok {
			b.addReadiness(assessment)
		}
		if result, ok := cat.Conformance[claim.ID]; ok {
			b.addConformance(result)
		}
		if b.exceeded {
			break
		}
	}
	if !b.exceeded {
		for key, ids := range cat.ByFacet {
			b.addString(key)
			for _, id := range ids {
				b.addString(id)
				b.add(64)
				if b.exceeded {
					break
				}
			}
			if b.exceeded {
				break
			}
		}
		for key, ids := range cat.ByModule {
			if b.exceeded {
				break
			}
			b.addString(key)
			for _, id := range ids {
				b.addString(id)
				b.add(64)
				if b.exceeded {
					break
				}
			}
		}
	}
	if b.exceeded {
		return b
	}
	return b
}

type catalogBudget struct {
	limit, total uint64
	exceeded     bool
}

func (b *catalogBudget) add(n uint64) {
	if b.exceeded || n > b.limit || b.total > b.limit-n {
		b.exceeded = true
		b.total = b.limit
		return
	}
	b.total += n
}

func (b *catalogBudget) addString(value string) {
	b.add(jsonQuotedUpperBound(value))
}

func jsonQuotedUpperBound(value string) uint64 {
	n := uint64(2)
	for i := 0; i < len(value); {
		c := value[i]
		if c < utf8.RuneSelf {
			switch c {
			case '\\', '"', '\b', '\f', '\n', '\r', '\t':
				n += 2
			case '<', '>', '&':
				n += 6
			default:
				if c < 0x20 {
					n += 6
				} else {
					n++
				}
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 1 {
			n += 6
			i++
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			n += 6
		} else {
			n += uint64(size)
		}
		i += size
	}
	return n
}

func (b *catalogBudget) addStrings(values []string) {
	for _, value := range values {
		b.addString(value)
		b.add(32)
		if b.exceeded {
			return
		}
	}
}

func (b *catalogBudget) addConformance(result conformance.Result) {
	b.add(4096)
	for _, value := range []string{result.ClaimID, string(result.Mode), result.Reason} {
		b.addString(value)
	}
	for _, check := range result.Checks {
		b.add(2048)
		for _, value := range []string{check.ID, check.Adapter, check.Target, string(check.Shape), string(check.State), check.Reason} {
			b.addString(value)
		}
		b.addConformanceValue(check.Expected)
		b.addConformanceValue(check.Observed)
		b.addStrings(check.Missing)
		b.addStrings(check.Extra)
		if check.ObservationError != nil {
			b.addString(check.ObservationError.Code)
			b.addString(check.ObservationError.Message)
		}
		if b.exceeded {
			return
		}
	}
}

func (b *catalogBudget) addConformanceValue(value any) {
	switch value := value.(type) {
	case nil:
	case string:
		b.addString(value)
	case []string:
		b.addStrings(value)
	default:
		// The conformance result contract is closed to string and []string.
		// Refuse an impossible in-memory value rather than underestimating it.
		b.exceeded = true
	}
}

func (b *catalogBudget) addReadiness(assessment readiness.Assessment) {
	b.add(4096)
	b.addString(assessment.ClaimID)
	b.addString(assessment.LocalApprovalIssue)
	b.addStrings(assessment.LocalReasons)
	addCondition := func(condition readiness.DependencyCondition) {
		b.add(256)
		b.addString(string(condition.Kind))
		b.addString(condition.DependencyID)
		b.addString(condition.Detail)
		b.addStrings(condition.Path)
	}
	for _, condition := range assessment.DependencyConditions {
		addCondition(condition)
		if b.exceeded {
			return
		}
	}
	for _, condition := range assessment.Conditions {
		addCondition(condition)
		if b.exceeded {
			return
		}
	}
	addCause := func(cause readiness.Cause) {
		b.add(320)
		b.addString(string(cause.Kind))
		b.addString(string(cause.SourceKind))
		b.addString(cause.DependencyID)
		b.addString(cause.Detail)
		b.addStrings(cause.Path)
	}
	for _, cause := range assessment.ReviewCauses {
		addCause(cause)
		if b.exceeded {
			return
		}
	}
	for _, cause := range assessment.Causes {
		addCause(cause)
		if b.exceeded {
			return
		}
	}
}

func encodeJSON(cat *Catalog, maxBytes int) ([]byte, error) {
	if cat == nil {
		cat = &Catalog{}
	}
	data, err := json.MarshalIndent(cat.Document(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("catalog: marshal: %w", err)
	}
	data = append(data, '\n')
	if maxBytes > 0 && len(data) > maxBytes {
		return nil, fmt.Errorf("%w: catalog output is %d bytes; maximum is %d", conformance.ErrCapacityExceeded, len(data), maxBytes)
	}
	return data, nil
}

// WriteEncoded atomically replaces one preflighted catalog artifact.
func WriteEncoded(path string, data []byte) error {
	if err := atomicfile.Write(path, data, 0o644); err != nil {
		return fmt.Errorf("catalog: write %q: %w", path, err)
	}
	return nil
}

// WriteJSON serializes cat's Document to path as indented JSON, creating
// path's parent directory if needed. Output is deterministic: building the
// same claims twice and writing both produces byte-identical files.
func WriteJSON(cat *Catalog, path string) error {
	data, err := EncodeJSON(cat)
	if err != nil {
		return err
	}
	return WriteEncoded(path, data)
}
